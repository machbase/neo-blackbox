package ffmpeg

import (
	"blackbox-backend/internal/config"
	"context"
	"strings"
)

type FFmpegRunner struct {
	cfg config.FFmpegConfig
}

func New(cfg config.FFmpegConfig) *FFmpegRunner {
	return &FFmpegRunner{
		cfg: cfg,
	}
}

func (r *FFmpegRunner) Run(ctx context.Context) error {

	return nil
}

func (r *FFmpegRunner) Args() []string {
	out := make([]string, 0, len(r.cfg.Cameras))
	for _, camera := range r.cfg.Cameras {
		input := flattenArgs(camera.InputArgs)
		inputStr := strings.Join(input, " ")

		output := flattenArgs(camera.OutputArgs)
		outputStr := strings.Join(output, " ")

		out = append(out, inputStr+outputStr)
	}
	return out
}

func flattenArgs(kvs []config.ArgKV) []string {
	out := make([]string, 0, len(kvs)*2)
	for _, arg := range kvs {
		if arg.Flag == "" {
			continue
		}
		out = append(out, arg.Flag)
		if arg.Value != "" {
			out = append(out, arg.Value)
		}
	}
	return out
}
