//go:build linux

package watcher

import (
	"blackbox-backend/internal/config"
	"blackbox-backend/internal/db"
	"blackbox-backend/internal/ffmpeg"
	"blackbox-backend/internal/logger"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const abnormal = unix.POLLHUP | unix.POLLERR | unix.POLLNVAL

type Watcher struct {
	cfg config.WatcherConfig

	neo     *db.Machbase
	ffRuner *ffmpeg.FFmpegRunner

	// 자주 사용하는 필드들
	RamDisk string
	DataDir string

	// 동적 watch 관리 (thread-safe)
	mu       sync.Mutex
	inFd     int
	watchSet *WatchSet
	mask     uint32
}

func New(cfg config.WatcherConfig, neo *db.Machbase, ffRunner *ffmpeg.FFmpegRunner) *Watcher {
	return &Watcher{
		cfg:     cfg,
		neo:     neo,
		ffRuner: ffRunner,
	}
}

type WatchSet struct {
	wdToRule   map[int32]config.WatcherRule // watch descriptor -> rule
	cameraToWd map[string]int32             // cameraID -> watch descriptor
}

func (ws *WatchSet) RemoveAll(inFd int) {
	for wd := range ws.wdToRule {
		_, _ = unix.InotifyRmWatch(inFd, uint32(wd))
	}
}

func (ws *WatchSet) Add(inFd int, rule config.WatcherRule, mask uint32) error {
	// 이미 해당 카메라가 등록되어 있는지 확인
	if wd, exists := ws.cameraToWd[rule.CameraID]; exists {
		return fmt.Errorf("camera %q already watching (wd=%d)", rule.CameraID, wd)
	}

	wd, err := unix.InotifyAddWatch(inFd, rule.SourceDir, mask)
	if err != nil {
		return fmt.Errorf("failed to inotify add watch(source_dir=%q): %v", rule.SourceDir, err)
	}

	ws.wdToRule[int32(wd)] = rule
	ws.cameraToWd[rule.CameraID] = int32(wd)
	return nil
}

func (ws *WatchSet) Remove(inFd int, cameraID string) error {
	wd, ok := ws.cameraToWd[cameraID]
	if !ok {
		return fmt.Errorf("camera %q not found in watch set", cameraID)
	}

	if _, err := unix.InotifyRmWatch(inFd, uint32(wd)); err != nil {
		return fmt.Errorf("failed to remove watch: %v", err)
	}

	delete(ws.wdToRule, wd)
	delete(ws.cameraToWd, cameraID)
	return nil
}

func (ws *WatchSet) GetRule(wd int32) (config.WatcherRule, bool) {
	rule, ok := ws.wdToRule[wd]
	return rule, ok
}

func (w *Watcher) addWatches(inFd int, rules []config.WatcherRule, mask uint32) (*WatchSet, error) {
	ws := WatchSet{
		wdToRule:   make(map[int32]config.WatcherRule, len(rules)),
		cameraToWd: make(map[string]int32, len(rules)),
	}

	for _, rule := range rules {
		if err := ws.Add(inFd, rule, mask); err != nil {
			// 하나의 rule이 실패해도 전체 종료 : 무시하고 나머지 rule 실행
			return nil, err
		}
	}

	return &ws, nil
}

type RuleFailure struct {
	id   int
	Rule config.WatcherRule
	Err  error
}

func (w *Watcher) prepare() ([]config.WatcherRule, []RuleFailure) {
	active := make([]config.WatcherRule, 0, len(w.cfg.Rules))
	failed := []RuleFailure{}

	for i, rule := range w.cfg.Rules {
		if rule.TargetDir == "" {
			reason := fmt.Errorf("target_dir is empty")
			logger.GetLogger().Infof("watcher rule[%d]: target_dir is empty (source_dir=%q)", i, rule.SourceDir)
			failed = append(failed, RuleFailure{id: i, Rule: rule, Err: reason})
			continue
		}
		if err := os.MkdirAll(rule.TargetDir, 0o755); err != nil {
			reason := fmt.Errorf("failed to mkdir target_dir=%q : %v,", rule.TargetDir, err)
			logger.GetLogger().Infof("watcher rule[%d]: mkdir target_dir=%q (source_dir=%q): %v", i, rule.TargetDir, rule.SourceDir, err)
			failed = append(failed, RuleFailure{id: i, Rule: rule, Err: reason})
			continue
		}
		active = append(active, rule)
	}

	return active, failed
}

func (w *Watcher) retryLoop(ctx context.Context, req chan int) {

}

func (w *Watcher) Run(ctx context.Context) error {
	logger.GetLogger().Infof("Start Watcher (rules len: %d)", len(w.cfg.Rules))

	active, _ := w.prepare() // active, failed

	inFd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return err
	}
	defer unix.Close(inFd)

	mask := uint32(unix.IN_CLOSE_WRITE | unix.IN_MOVED_TO)

	watchSet, err := w.addWatches(inFd, active, mask)
	if err != nil {
		return err
	}

	// Store in watcher for dynamic add/remove
	w.mu.Lock()
	w.inFd = inFd
	w.watchSet = watchSet
	w.mask = mask
	w.mu.Unlock()

	wakeFd, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		return err
	}
	defer unix.Close(wakeFd)

	go func() {
		<-ctx.Done()
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], 1)
		_, _ = unix.Write(wakeFd, b[:])
	}()

	pfds := []unix.PollFd{
		{Fd: int32(inFd), Events: unix.POLLIN},
		{Fd: int32(wakeFd), Events: unix.POLLIN},
	}

	buf := make([]byte, 64*1024)

	for {
		_, err := unix.Poll(pfds, -1)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return err
		}

		reWake := pfds[1]
		if reWake.Revents&abnormal != 0 {
			return fmt.Errorf("wakeFd abnormal revents=%#x", reWake.Revents)
		}
		if reWake.Revents&unix.POLLIN != 0 {
			var tmp [8]byte
			_, _ = unix.Read(wakeFd, tmp[:])
			watchSet.RemoveAll(inFd)
			logger.GetLogger().Info("Stop Watcher")
			return nil
		}

		reIn := pfds[0]
		if reIn.Revents&unix.POLLIN != 0 {
			for {
				n, err := unix.Read(inFd, buf)
				if err != nil {
					if err == unix.EAGAIN {
						break
					}
					if err == unix.EINTR {
						continue
					}
					return err
				}

				if n <= 0 {
					break
				}

				parseInotifyEvents(buf[:n], func(ev unix.InotifyEvent, name string) {
					w.mu.Lock()
					rule, ok := watchSet.GetRule(ev.Wd)
					w.mu.Unlock()

					if !ok {
						return
					}
					if err := w.handleEvent(ctx, ev, name, rule); err != nil {
						logger.GetLogger().Infof("handleEvent: %v", err)
						// return fmt.Errorf("handleEvent: %v", err)
					}
				})
			}
		}
		if reIn.Revents&abnormal != 0 {
			return fmt.Errorf("inFd abnormal revents=%#x", reIn.Revents)
		}
	}
}

func parseInotifyEvents(b []byte, fn func(ev unix.InotifyEvent, name string)) {
	const sz = unix.SizeofInotifyEvent

	for len(b) >= sz {
		raw := b[:sz]
		evVal := *(*unix.InotifyEvent)(unsafe.Pointer(&raw[0]))

		nameLen := int(evVal.Len)
		name := ""
		if nameLen > 0 && len(b) >= sz+nameLen {
			nb := b[sz : sz+nameLen]
			if i := bytes.IndexByte(nb, 0); i >= 0 {
				nb = nb[:i]
			}
			name = string(nb)
		}

		fn(evVal, name)

		step := sz + nameLen
		if step > len(b) {
			return
		}

		b = b[step:]
	}
}

// 특정 이름으로 처리하는 방식은 나중에 필요할때 추가
func (w *Watcher) handleEvent(ctx context.Context, ev unix.InotifyEvent, name string, rule config.WatcherRule) error {
	if name == "" {
		return nil
	}
	if !strings.EqualFold(filepath.Ext(name), rule.Ext) {
		return nil
	}
	if ev.Mask&unix.IN_MOVED_TO == 0 {
		return nil
	}

	if err := w.syncInit(rule); err != nil {
		return fmt.Errorf("syncInit: %v", err)
	}

	base := filepath.Base(name)

	switch {
	case strings.HasPrefix(base, "init"):
		return nil

	case strings.HasPrefix(base, "chunk-stream"):
		return w.proecessChunk(ctx, rule, base)

	default:
		return nil
	}
}

func (w *Watcher) syncInit(rule config.WatcherRule) error {
	srcPattern := filepath.Join(rule.SourceDir, "init*"+rule.Ext)
	srcInits, err := filepath.Glob(srcPattern)
	if err != nil {
		return err
	}
	if len(srcInits) == 0 {
		return nil
	}

	destPattern := filepath.Join(rule.TargetDir, "init*"+rule.Ext)
	destInits, _ := filepath.Glob(destPattern)
	for _, p := range destInits {
		_ = os.Remove(p)
	}

	if err := os.MkdirAll(rule.TargetDir, 0o755); err != nil {
		return err
	}

	for _, src := range srcInits {
		dst := filepath.Join(rule.TargetDir, filepath.Base(src))
		if err := moveFile(src, dst); err != nil {
			return fmt.Errorf("move init %q -> %q: %v", src, dst, err)
		}
		logger.GetLogger().Infof("[INIT] moved: %s -> %s", src, dst)
	}
	return nil
}

func extraChunkPrefix(filename, ext string) (string, error) {
	base := filepath.Base(filename)

	if ext == "" {
		ext = filepath.Ext(base)
	}
	if ext == "" {
		return "", fmt.Errorf("no extension in %q", base)
	}

	if !strings.EqualFold(filepath.Ext(base), ext) {
		return "", fmt.Errorf("invalid ext: name=%q ext=%q want=%q", base, filepath.Ext(base), ext)
	}

	stem := strings.TrimSuffix(base, filepath.Ext(base))

	i := strings.LastIndexByte(stem, '-')
	if i < 0 || i == len(stem)-1 {
		return "", fmt.Errorf("invalid chunk name (no numeric suffix): %q", base)
	}

	suffix := stem[i+1:]
	if !isAllDigits(suffix) {
		return "", fmt.Errorf("invalid chunk name (suffix not digits): %q", base)
	}

	prefix := stem[:i+1]
	if prefix == "" {
		return "", fmt.Errorf("invalid chunk name (empty prefix): %q", base)
	}

	return prefix, nil
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || '9' < c {
			return false
		}
	}
	return len(s) > 0
}

func (w *Watcher) proecessChunk(ctx context.Context, rule config.WatcherRule, name string) error {
	name = filepath.Base(name)

	observedEpochMs := time.Now().UnixMilli()

	// prefix 추출 + 새 이름 생성
	prefix, err := extraChunkPrefix(name, rule.Ext)
	if err != nil {
		return err
	}
	newName := prefix + strconv.FormatInt(observedEpochMs, 10) + rule.Ext

	// src -> dest 이동 (새 이름)
	srcPath := filepath.Join(rule.SourceDir, name)
	tmpDestPath := filepath.Join(rule.TargetDir, newName)

	if err := os.MkdirAll(rule.TargetDir, 0o755); err != nil {
		return err
	}
	if err := moveFile(srcPath, tmpDestPath); err != nil {
		return fmt.Errorf("move chunk %q -> %q: %v", srcPath, tmpDestPath, err)
	}

	// chunk 크기 검증
	if ok, err := checkFileMinSize(tmpDestPath, 1000); !ok || err != nil {
		return fmt.Errorf("chunk invalid: %v", err)
	}

	initPath := filepath.Join(rule.TargetDir, "init-stream0"+rule.Ext)
	if ok, err := checkFileMinSize(initPath, 100); !ok || err != nil {
		return fmt.Errorf("init invalid: %v", err)
	}

	timing, err := w.ffRuner.ProbeConcatPacketTiming(ctx, initPath, tmpDestPath)
	if err != nil {
		return fmt.Errorf("ProbeConcatPacketTiming: %v", err)
	}

	if timing.Length < 0 {
		return fmt.Errorf("negative length: start=%.6f last=%.6f dur=%.6f len=%.6f",
			timing.StartPTS, timing.LastPTS, timing.LastDur, timing.Length,
		)
	}

	// 날짜 디렉토리 생성
	dateDir := time.UnixMilli(observedEpochMs).UTC().Format("20060102")
	finalDir := filepath.Join(rule.TargetDir, dateDir)
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return fmt.Errorf("mkdir date dir %q: %v", finalDir, err)
	}

	// out 루트 -> 날짜 디렉토리로 이동
	finalPath := filepath.Join(finalDir, newName)
	if err := moveFile(tmpDestPath, finalPath); err != nil {
		return fmt.Errorf("move into date dir %q -> %q: %v", tmpDestPath, finalPath, err)
	}

	startNs := int64(math.Round(timing.StartPTS * 1e9))
	lengthNs := int64(math.Round(timing.Length * 1e9))

	if err := w.neo.InsertChunk(ctx, rule.CameraID, startNs, lengthNs, observedEpochMs); err != nil {
		return fmt.Errorf("InsertChunk: %v", err)
	}

	logger.GetLogger().Infof("[CHUNK] %s -> %s start=%.6f len=%.6f epochMs=%d", name, finalPath, timing.StartPTS, timing.Length, observedEpochMs)

	return nil
}

func moveFile(src, dst string) error {
	_ = os.Remove(dst)

	if err := os.Rename(src, dst); err == nil {
		return nil
	} else {
		// logger.GetLogger().Infof()
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}

	if err := os.Remove(src); err != nil {
		return err
	}

	return nil
}

func checkFileMinSize(path string, minSize int64) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("%q is not exist", path)
		}
		return false, err
	}
	if info.IsDir() {
		// log
		return false, fmt.Errorf("%q is directory", path)
	}
	if info.Size() < minSize {
		// log
		return false, fmt.Errorf("%q size(%d) is too small (<%d)", path, info.Size(), minSize)
	}
	return true, nil
}

// AddWatch dynamically adds a new watch rule to the running watcher.
// This is called when a camera is enabled via API.
func (w *Watcher) AddWatch(ctx context.Context, rule config.WatcherRule) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watchSet == nil {
		return fmt.Errorf("watcher not running")
	}

	// Prepare target directory
	if rule.TargetDir == "" {
		return fmt.Errorf("target_dir is empty")
	}
	if err := os.MkdirAll(rule.TargetDir, 0o755); err != nil {
		return fmt.Errorf("failed to mkdir target_dir=%q: %v", rule.TargetDir, err)
	}

	// Add to inotify watch set
	if err := w.watchSet.Add(w.inFd, rule, w.mask); err != nil {
		return err
	}

	logger.GetLogger().Infof("[watcher] added watch: camera_id=%s source_dir=%s", rule.CameraID, rule.SourceDir)
	return nil
}

// RemoveWatch dynamically removes a watch rule from the running watcher.
// This is called when a camera is disabled via API.
func (w *Watcher) RemoveWatch(ctx context.Context, cameraID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watchSet == nil {
		return fmt.Errorf("watcher not running")
	}

	if err := w.watchSet.Remove(w.inFd, cameraID); err != nil {
		return err
	}

	logger.GetLogger().Infof("[watcher] removed watch: camera_id=%s", cameraID)
	return nil
}
