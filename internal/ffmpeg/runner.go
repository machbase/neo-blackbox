package ffmpeg

import (
	"blackbox-backend/internal/config"
	"context"
)

type FFmpegRunner struct {
	cfg config.FFmpegConfig
}

func New(cfg config.FFmpegConfig) *FFmpegRunner {
	return &FFmpegRunner{
		cfg: cfg,
	}
}

func (r *FFmpegRunner) Args() []string {
	return []string{}
}

func (r *FFmpegRunner) Run(ctx context.Context) error {
	// exec.CommandContext(ctx, "ffmpeg")
	return nil
}
