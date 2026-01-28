package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"blackbox-backend/internal/config"
	"blackbox-backend/internal/db"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server.
type Server struct {
	cfg     config.ServerConfig
	engine  *gin.Engine
	http    *http.Server
	handler *Handler
}

// New creates a new Server.
func New(cfg config.ServerConfig, machbase *db.Machbase) (*Server, error) {
	cfg.ApplyDefaults()

	if cfg.BaseDir == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		cfg.BaseDir = filepath.Dir(exe)
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(cors())

	s := &Server{
		cfg:     cfg,
		engine:  engine,
		handler: NewHandler(machbase, cfg.DataPath),
	}
	s.routes()

	s.http = &http.Server{
		Addr:         cfg.Addr,
		Handler:      engine,
		ReadTimeout:  cfg.ReadTimeout(),
		WriteTimeout: cfg.WriteTimeout(),
	}

	return s, nil
}

func (s *Server) routes() {
	api := s.engine.Group("/api")

	// Blackbox
	api.GET("/cameras", s.handler.GetCameras)
	api.GET("/get_time_range", s.handler.GetTimeRange)
	api.GET("/get_chunk_info", s.handler.GetChunkInfo)
	api.GET("/v_get_chunk", s.handler.GetChunk)
	api.GET("/get_camera_rollup_info", s.handler.GetCameraRollup)

	// Sensor
	api.GET("/sensors", s.handler.GetSensors)
	api.GET("/sensor_data", s.handler.GetSensorData)

	// Static files
	s.engine.StaticFS("/", http.Dir(s.cfg.BaseDir))
}

// Run starts the server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on http://%s", s.cfg.Addr)
		if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout())
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-Machbase-Api-Token, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}
