package watcher

import (
	"blackbox-backend/internal/config"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

type Watcher struct {
	cfg config.WatcherConfig

	// 자주 사용하는 필드들
	RamDisk string
	DataDir string
}

func New(cfg config.WatcherConfig) *Watcher {
	return &Watcher{
		cfg: cfg,
	}
}

type WatchSet struct {
	wdToIdx map[int32]int
	wds     map[int32]struct{}
}

func (ws *WatchSet) RemoveAll(inFd int) {
	for wd := range ws.wds {
		_, _ = unix.InotifyRmWatch(inFd, uint32(wd))
	}
}

func (w *Watcher) addWatches(inFd int, rules []config.WatcherRule, mask uint32) (*WatchSet, error) {
	ws := WatchSet{
		wdToIdx: make(map[int32]int, len(rules)),
		wds:     make(map[int32]struct{}, len(rules)),
	}

	for i, rule := range rules {
		wd, err := unix.InotifyAddWatch(inFd, rule.SourceDir, mask)
		if err != nil {
			// 하나의 rule이 실패해도 전체 종료 : 무시하고 나머지 rule 실행
			return nil, fmt.Errorf("failed to inotify add watch(source_dir=%q): %v", rule.SourceDir, err)
		}

		ws.wdToIdx[int32(wd)] = i
		ws.wds[int32(wd)] = struct{}{}
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
			log.Printf("watcher rule[%d]: target_dir is empty (source_dir=%q)", i, rule.SourceDir)
			failed = append(failed, RuleFailure{id: i, Rule: rule, Err: reason})
			continue
		}
		if err := os.MkdirAll(rule.TargetDir, 0o755); err != nil {
			reason := fmt.Errorf("failed to mkdir target_dir=%q : %v,", rule.TargetDir, err)
			log.Printf("watcher rule[%d]: mkdir target_dir=%q (source_dir=%q): %v", i, rule.TargetDir, rule.SourceDir, err)
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
	log.Printf("Start Watcher (rules len: %d)", len(w.cfg.Rules))

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

	buf := make([]byte, 64*1024)

	pfds := []unix.PollFd{
		{Fd: int32(inFd), Events: unix.POLLIN},
		{Fd: int32(wakeFd), Events: unix.POLLIN},
	}

	const abnormal = unix.POLLHUP | unix.POLLERR | unix.POLLNVAL

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
			log.Println("Stop Watcher")
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
					idx, ok := watchSet.wdToIdx[ev.Wd]
					if !ok {
						return
					}
					rule := w.cfg.Rules[idx]
					w.handleEvent(ev, name, rule)
				})
			}
		}
		if reIn.Revents&abnormal != 0 {
			return fmt.Errorf("inFd abnormal revents=%#x", reIn.Revents)
		}
	}
}

func (w *Watcher) handleEvent(ev unix.InotifyEvent, name string, rule config.WatcherRule) {
	if ev.Mask&unix.IN_CLOSE_WRITE != 0 {
		if strings.EqualFold(filepath.Ext(name), rule.Ext) {
			sourceFile := filepath.Join(rule.SourceDir, name)
			targetFile := filepath.Join(rule.TargetDir, name)

			os.Rename(sourceFile)

			log.Printf("CLOSE_WRITE (ext:%s): %s ---> %s", rule.Ext, sourceFile, targetFile)
		} else {
			log.Println("CLOSE_WRITE : ", name)
		}
	}

	if ev.Mask&unix.IN_MOVED_TO != 0 {
		log.Println("MOVED_TO: ", name)
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
