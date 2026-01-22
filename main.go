package main

import (
	"blackbox-backend/internal/config"
	"blackbox-backend/internal/ffmpeg"
	"blackbox-backend/internal/watcher"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
		fmt.Fprintf(os.Stderr, "blackbox-backend: %v", err)
		os.Exit(1)
	}
}

func run(c context.Context, path string) error {
	ctx, cancel := signal.NotifyContext(c, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	ff := ffmpeg.New(cfg.FFmpeg)
	w := watcher.New(cfg.Watcher)
	// s := server.New(cfg.Server)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return w.Run(gctx)
	})

	g.Go(func() error {
		return ff.Run(gctx)
	})

	// g.Go(func() error {
	// return svr.Run()
	// })

	// g.Go(func() error {
	// <-gctx.Done()
	// return  srv.Shutdown(context.Background())
	// })

	// g.Go로 실행한 모든 고루틴들이 종료될때까지 g.Wait() 대기
	if err := g.Wait(); err != nil {
		cancel()
		return err
	}

	return nil
}
