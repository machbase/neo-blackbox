package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"blackbox-backend/internal/config"
	"blackbox-backend/internal/db"
	"blackbox-backend/internal/logger"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server.
type Server struct {
	cfg     config.ServerConfig
	engine  *gin.Engine
	http    *http.Server
	handler *Handler
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.engine.ServeHTTP(w, req)
}

// New creates a new Server.
func New(cfg config.ServerConfig, machbase *db.Machbase, watcher Watcher, ffmpegBinary ...string) (*Server, error) {
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

	var ffBinary string
	if len(ffmpegBinary) > 0 {
		ffBinary = ffmpegBinary[0]
	}

	s := &Server{
		cfg:     cfg,
		engine:  engine,
		handler: NewHandler(machbase, watcher, cfg.DataDir, cfg.MvsDir, cfg.CameraDir, ffBinary),
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

	// ==================================================================
	// Blackbox
	api.GET("/cameras", s.handler.GetCameras)
	api.GET("/get_time_range", s.handler.GetTimeRange)
	api.GET("/get_chunk_info", s.handler.GetChunkInfo)
	api.GET("/v_get_chunk", s.handler.GetChunk)
	api.GET("/get_camera_rollup_info", s.handler.GetCameraRollup)

	// Sensor
	api.GET("/sensors", s.handler.GetSensors)
	api.GET("/sensor_data", s.handler.GetSensorData)
	// ==================================================================
	// Camera Management
	api.POST("/camera", s.handler.CreateCamera)       // O
	api.GET("/camera/:id", s.handler.GetCamera)       // O
	api.POST("/camera/:id", s.handler.UpdateCamera)   // O
	api.DELETE("/camera/:id", s.handler.DeleteCamera) // O
	api.POST("/camera/test", s.handler.TestCameraConnection)

	// Camera Control
	api.POST("/camera/:id/enable", s.handler.EnableCamera)   // O
	api.POST("/camera/:id/disable", s.handler.DisableCamera) // O

	// Camera Status Monitoring
	api.GET("/camera/:id/status", s.handler.GetCameraStatus) // O
	api.GET("/cameras/health", s.handler.GetCamerasHealth)   // O

	// ==================================================================
	// Event Rule
	api.GET("/event_rule", s.handler.GetEventRules)
	api.POST("/event_rule", s.handler.PostEventRules)
	api.PUT("/event_rule", s.handler.UpdateEventRules)
	api.DELETE("/event_rule", s.handler.DeleteEventRules)

	// ==================================================================
	// AI
	api.POST("/ai/result", s.handler.UploadAIResult)
	// ==================================================================

	// Web UI - Serve static frontend
	webDir := filepath.Join(s.cfg.BaseDir, "web")
	s.engine.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(webDir, "index.html"))
	})
	s.engine.StaticFS("/web", http.Dir(webDir))
}

// Run starts the server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		logger.GetLogger().Infof("listening on http://%s", s.cfg.Addr)
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
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}

type Response struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    any    `json:"data"`
}
