package server

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"neo-blackbox/internal/config"

	"github.com/gin-gonic/gin"
)

// AppConfigRequest는 POST/GET /api/config 에서 사용하는 설정 구조체.
// log, ai, mediamtx, ffmpeg.defaults 는 고정값으로 관리되며 API에서 제외.
type AppConfigRequest struct {
	Server   ServerConfigAPI   `json:"server"`
	Machbase MachbaseConfigAPI `json:"machbase"`
	Ffmpeg   FfmpegConfigAPI   `json:"ffmpeg"`
}

type ServerConfigAPI struct {
	Addr      string `json:"addr"`
	CameraDir string `json:"camera_dir"`
	MvsDir    string `json:"mvs_dir"`
	DataDir   string `json:"data_dir"`
}

type MachbaseConfigAPI struct {
	Disabled       bool   `json:"disabled"`
	Scheme         string `json:"scheme"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	APIToken       string `json:"api_token"`
	User           string `json:"user"`
	Password       string `json:"password"`
}

type FfmpegConfigAPI struct {
	Binary string `json:"binary"`
}

// fixedDefaults returns the hardcoded fixed sections of the config
// (log, ai, mediamtx, ffmpeg.defaults). These are always written as-is.
func fixedDefaults() config.AppConfig {
	return config.AppConfig{
		Mediamtx: config.MediamtxConfig{
			Binary:     "../tools/mediamtx",
			ConfigFile: "../tools/mediamtx.yml",
			Host:       "127.0.0.1",
			Port:       9997,
		},
		FFmpeg: config.FFmpegConfig{
			Defaults: config.FFmpegDefaults{
				ProbeBinary: "../tools/ffprobe",
			},
		},
		AI: config.AIConfig{
			Binary:     "../ai/blackbox-ai-manager",
			ConfigFile: "../ai/config.json",
		},
		Log: config.LogConfig{
			Dir:    "../logs",
			Level:  "info",
			Format: "json",
			Output: "both",
			File: config.LogFileConfig{
				Filename:   "blackbox.log",
				MaxSize:    100,
				MaxBackups: 10,
				MaxAge:     30,
				Compress:   true,
			},
		},
	}
}

// GetAppConfig godoc
// GET /api/config
// config.yaml 에서 server, machbase, ffmpeg.binary 를 반환한다.
func (h *Handler) GetAppConfig(c *gin.Context) {
	tick := time.Now()

	cfg, err := config.LoadRaw(h.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 파일 없으면 빈 구조체 반환
			successResponse(c, tick, AppConfigRequest{})
			return
		}
		errorResponse(c, tick, http.StatusInternalServerError, "failed to read config: "+err.Error())
		return
	}

	successResponse(c, tick, AppConfigRequest{
		Server: ServerConfigAPI{
			Addr:      cfg.Server.Addr,
			CameraDir: cfg.Server.CameraDir,
			MvsDir:    cfg.Server.MvsDir,
			DataDir:   cfg.Server.DataDir,
		},
		Machbase: MachbaseConfigAPI{
			Disabled:       cfg.Machbase.Disabled,
			Scheme:         cfg.Machbase.Scheme,
			Host:           cfg.Machbase.Host,
			Port:           cfg.Machbase.Port,
			TimeoutSeconds: cfg.Machbase.TimeoutSeconds,
			APIToken:       cfg.Machbase.APIToken,
			User:           cfg.Machbase.User,
			Password:       cfg.Machbase.Password,
		},
		Ffmpeg: FfmpegConfigAPI{
			Binary: cfg.FFmpeg.Binary,
		},
	})
}

// PostAppConfig godoc
// POST /api/config
// server, machbase, ffmpeg.binary 를 받아 config.yaml 을 생성/갱신한다.
// log, ai, mediamtx, ffmpeg.defaults 는 고정값으로 자동 기입된다.
func (h *Handler) PostAppConfig(c *gin.Context) {
	tick := time.Now()

	var req AppConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// 고정 섹션(log, ai, mediamtx, ffmpeg.defaults)으로 시작
	cfg := fixedDefaults()

	// 사용자 설정 섹션 적용
	cfg.Server = config.ServerConfig{
		Addr:      req.Server.Addr,
		CameraDir: req.Server.CameraDir,
		MvsDir:    req.Server.MvsDir,
		DataDir:   req.Server.DataDir,
	}
	cfg.Machbase = config.MachbaseConfig{
		Disabled:       req.Machbase.Disabled,
		Scheme:         req.Machbase.Scheme,
		Host:           req.Machbase.Host,
		Port:           req.Machbase.Port,
		TimeoutSeconds: req.Machbase.TimeoutSeconds,
		APIToken:       req.Machbase.APIToken,
		User:           req.Machbase.User,
		Password:       req.Machbase.Password,
	}
	// ffmpeg.defaults 는 fixedDefaults 에서 이미 설정됨
	cfg.FFmpeg.Binary = req.Ffmpeg.Binary

	// config 디렉토리 생성 (없으면)
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0755); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to create config dir: "+err.Error())
		return
	}

	if err := config.Save(h.configPath, &cfg); err != nil {
		errorResponse(c, tick, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	successResponse(c, tick, nil)
}
