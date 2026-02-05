package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"blackbox-backend/internal/config"
	"blackbox-backend/internal/db"
	"blackbox-backend/internal/ffmpeg"
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

	neo, err := db.NewMachbase(cfg.Machbase)
	if err != nil {
		return fmt.Errorf("create machbase client: %w", err)
	}

	svr, err := server.New(cfg.Server, neo, cfg.FFmpeg.Binary)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	ff := ffmpeg.New(cfg.FFmpeg)
	w := watcher.New(cfg.Watcher, neo, ff)

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
		log.Printf("shutdown: %v", err)
	}

	return nil
}
