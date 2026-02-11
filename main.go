package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"path/filepath"

	"blackbox-backend/internal/config"
	"blackbox-backend/internal/db"
	"blackbox-backend/internal/ffmpeg"
	"blackbox-backend/internal/logger"
	"blackbox-backend/internal/server"
	"blackbox-backend/internal/watcher"

	"golang.org/x/sync/errgroup"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "", "Path to the YAML configuration file (e.g., ./config.yaml)")
	flag.Parse()

	if configFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(context.Background(), configFile); err != nil {
		fmt.Fprintf(os.Stderr, "blackbox-backend: %v\n", err)
		os.Exit(1)
	}
}

func run(c context.Context, path string) error {
	ctx, cancel := signal.NotifyContext(c, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Initialize logger
	if err := logger.Init(cfg.Log); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	log := logger.GetLogger()

	neo, err := db.NewMachbase(cfg.Machbase)
	if err != nil {
		return fmt.Errorf("create machbase client: %w", err)
	}

	logDir := cfg.Log.Dir
	if logDir == "" {
		logDir = filepath.Dir(cfg.Log.File.Filename)
	}

	// Initialize HTTP logger
	if err := logger.InitHTTPLogger(logDir); err != nil {
		return fmt.Errorf("init http logger: %w", err)
	}
	ff := ffmpeg.New(cfg.FFmpeg, logDir)
	w := watcher.New(neo, ff, cfg.Server.CameraDir)

	svr, err := server.New(cfg.Server, cfg.Mediamtx, logDir, neo, w, cfg.FFmpeg.Binary)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return w.Run(gctx)
	})

	// g.Go(func() error {
	// 	return ff.Run(gctx)
	// })

	g.Go(func() error {
		return svr.Run(gctx)
	})

	if err := g.Wait(); err != nil {
		log.Warnf("shutdown: %v", err)
	}

	log.Info("blackbox-backend stopped gracefully")
	return nil
}
