package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"path/filepath"

	"neo-blackbox/internal/ai"
	"neo-blackbox/internal/config"
	"neo-blackbox/internal/db"
	"neo-blackbox/internal/ffmpeg"
	"neo-blackbox/internal/logger"
	"neo-blackbox/internal/mediamtx"
	"neo-blackbox/internal/server"
	"neo-blackbox/internal/watcher"

	"golang.org/x/sync/errgroup"
)

func main() {
	var configFile string
	var serveWeb bool
	flag.StringVar(&configFile, "config", "", "Path to the YAML configuration file (e.g., ./config.yaml)")
	flag.BoolVar(&serveWeb, "web", false, "Serve web UI from {basedir}/web (default: false)")
	flag.Parse()

	if configFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(context.Background(), configFile, serveWeb); err != nil {
		fmt.Fprintf(os.Stderr, "neo-blackbox: %v\n", err)
		os.Exit(1)
	}
}

func run(c context.Context, path string, serveWeb bool) error {
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

	svr, err := server.New(cfg.Server, cfg.Mediamtx, logDir, neo, w, ff, cfg.FFmpeg.Binary, serveWeb)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	mediamtxRunner := mediamtx.New(mediamtx.Config{Binary: cfg.Mediamtx.Binary, ConfigFile: cfg.Mediamtx.ConfigFile}, logDir)
	aiMgr := ai.New(cfg.AI, logDir)

	g, gctx := errgroup.WithContext(ctx)

	// mediaMTX
	g.Go(func() error {
		return mediamtxRunner.Run(gctx)
	})

	// ai-manager
	g.Go(func() error {
		return aiMgr.Run(gctx)
	})

	// watcher
	g.Go(func() error {
		return w.Run(gctx)
	})

	// web-server
	g.Go(func() error {
		return svr.Run(gctx)
	})

	if err := g.Wait(); err != nil {
		log.Warnf("shutdown: %v", err)
	}

	log.Info("neo-blackbox stopped gracefully")
	return nil
}
