package ffmpeg

import (
	"blackbox-backend/internal/config"
	"blackbox-backend/internal/logger"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type FFmpegRunner struct {
	cfg    config.FFmpegConfig
	logDir string
}

func New(cfg config.FFmpegConfig, logDir string) *FFmpegRunner {
	return &FFmpegRunner{
		cfg:    cfg,
		logDir: logDir,
	}
}

type CamEvent struct {
	ID    string
	Index int
	Stage string
	Err   error
}

func (r *FFmpegRunner) Run(ctx context.Context) error {
	wg := sync.WaitGroup{}
	events := make(chan CamEvent, len(r.cfg.Cameras))

	go func() {
		for ev := range events {
			if ev.Err != nil {
				logger.GetLogger().Debugf("[%d:%s] %s error: %v", ev.Index, ev.ID, ev.Stage, ev.Err)
			} else {
				logger.GetLogger().Debugf("[%d:%s] %s", ev.Index, ev.ID, ev.Stage)
			}
		}
	}()

	for i, cam := range r.cfg.Cameras {
		wg.Add(1)
		go func(i int, cam config.CameraJob) {
			defer wg.Done()

			execArgs := r.buildExecArgs(cam)

			// Resolve ffmpeg binary: use config if set, otherwise system PATH
			ffmpegBin := "ffmpeg"
			if r.cfg.Binary != "" {
				ffmpegBin = r.cfg.Binary
			}

			logger.GetLogger().Debugf("FFmpeg command:\n%s\n", prettyCommand(ffmpegBin, execArgs))

			cmd := exec.CommandContext(ctx, ffmpegBin, execArgs...)
			cmd.Dir = cam.OutputDIR

			if r.logDir != "" {
				logPath := filepath.Join(r.logDir, fmt.Sprintf("ffmpeg-%s.log", cam.ID))
				if err := os.MkdirAll(r.logDir, 0o755); err != nil {
					events <- CamEvent{ID: cam.ID, Index: i, Stage: "log-init", Err: err}
					return
				}
				logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
				if err != nil {
					events <- CamEvent{ID: cam.ID, Index: i, Stage: "log-init", Err: err}
					return
				}
				defer logFile.Close()
				cmd.Stdout = logFile
				cmd.Stderr = logFile
			} else {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			}

			if err := cmd.Run(); err != nil {
				if ctx.Err() != nil {
					events <- CamEvent{ID: cam.ID, Index: i, Stage: "exit", Err: err}
					return
				}
				events <- CamEvent{ID: cam.ID, Index: i, Stage: "exit", Err: err}
				return
			}

			events <- CamEvent{ID: cam.ID, Index: i, Stage: "exit", Err: nil}
		}(i, cam)
	}

	wg.Wait()
	close(events)
	return nil
}

func (r *FFmpegRunner) buildExecArgs(camera config.CameraJob) []string {
	args := []string{}
	args = append(args, flattenExecArgs(camera.InputArgs)...)
	args = append(args, "-i", camera.RtspURL)
	args = append(args, flattenExecArgs(camera.MidArgs)...)
	args = append(args, flattenExecArgs(camera.OutputArgs)...)
	args = append(args, camera.OutputName) // manifest.mpd
	return args
}

type SegmentTiming struct {
	StartPTS float64
	LastPTS  float64
	LastDur  float64
	Length   float64
}

func (r *FFmpegRunner) ProbeConcatPacketTiming(ctx context.Context, initFile string, chunkFile string) (SegmentTiming, error) {
	probeArgs := r.buildProbeArgs(initFile, chunkFile)

	// Resolve ffprobe binary: use config if set, otherwise system PATH
	ffprobeBin := "ffprobe"
	if r.cfg.Defaults.ProbeBinary != "" {
		ffprobeBin = r.cfg.Defaults.ProbeBinary
	}

	logger.GetLogger().Debugf("ffprobe command: %s\n", prettyCommand(ffprobeBin, probeArgs))

	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, ffprobeBin, probeArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SegmentTiming{}, err
	}
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return SegmentTiming{}, err
	}

	sc := bufio.NewScanner(stdout)
	var firstLine, lastLine string
	for sc.Scan() {
		line := sc.Text()
		if firstLine == "" {
			firstLine = line
		}
		lastLine = line
	}
	scErr := sc.Err()

	waitErr := cmd.Wait()
	if scErr != nil {
		return SegmentTiming{}, scErr
	}
	if waitErr != nil {
		return SegmentTiming{}, fmt.Errorf("ffprobe failed: %v, stderr=%s", waitErr, errBuf.String())
	}
	if firstLine == "" || lastLine == "" {
		return SegmentTiming{}, fmt.Errorf("ffprobe produced no packet lines; stderr=%s", errBuf.String())
	}

	start, err := parseCSVFloat(firstLine, 0)
	lastPts, err := parseCSVFloat(lastLine, 0)
	lastDur, err := parseCSVFloat(lastLine, 1)
	length := (lastPts + lastDur) - start

	return SegmentTiming{
		StartPTS: start,
		LastPTS:  lastPts,
		LastDur:  lastDur,
		Length:   length,
	}, nil
}

func parseCSVFloat(line string, field int) (float64, error) {
	a, b, ok := strings.Cut(line, ",")
	if !ok {
		return 0, fmt.Errorf("bad csv line: %q", line)
	}

	var s string
	if field == 0 {
		s = a
	} else {
		s = b
	}

	if s == "N/A" || s == "" {
		return 0, fmt.Errorf("missing value in line: %q", line)
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float %q: %v", s, err)
	}

	return v, nil
}

func (r *FFmpegRunner) buildProbeArgs(initFile string, chunkFile string) []string {
	args := []string{}
	args = append(args, flattenExecArgs(r.cfg.Defaults.ProbeArgs)...)
	args = append(args, fmt.Sprintf("concat:%s|%s", initFile, chunkFile))
	return args
}

func flattenExecArgs(kvs []config.ArgKV) []string {
	out := make([]string, 0, len(kvs)*2)
	for _, arg := range kvs {
		if arg.Flag == "" {
			continue
		}
		flag := arg.Flag
		if !strings.HasPrefix(arg.Flag, "-") {
			flag = "-" + flag
		}
		out = append(out, flag)

		if arg.Value != "" {
			out = append(out, arg.Value)
		}
	}
	return out
}

func prettyCommand(b string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(b))

	for _, a := range args {
		qa := shellQuote(a)

		if strings.HasPrefix(a, "-") {
			parts = append(parts, " \\\n"+qa)
		} else {
			parts = append(parts, " "+qa)
		}
	}

	return strings.Join(parts, "")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}

	need := false
	for _, c := range s {
		if c == ' ' || c == '\n' || c == '\t' || c == '\'' || c == '"' || c == '\\' {
			need = true
			break
		}
	}

	if !need {
		return s
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
