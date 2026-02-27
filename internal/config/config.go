package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	FFmpeg   FFmpegConfig   `yaml:"ffmpeg"`
	Server   ServerConfig   `yaml:"server"`
	Machbase MachbaseConfig `yaml:"machbase"`
	Mediamtx MediamtxConfig `yaml:"mediamtx"`
	AI       AIConfig       `yaml:"ai"`
	Log      LogConfig      `yaml:"log"`
}

type LogConfig struct {
	Dir    string        `yaml:"dir"`    // log directory (all log files go here)
	Level  string        `yaml:"level"`  // debug, info, warn, error, fatal, panic
	Format string        `yaml:"format"` // json or text
	Output string        `yaml:"output"` // stdout, file, both
	File   LogFileConfig `yaml:"file"`
}

type LogFileConfig struct {
	Filename   string `yaml:"filename"`
	MaxSize    int    `yaml:"max_size"`    // MB
	MaxBackups int    `yaml:"max_backups"` // 백업 파일 개수
	MaxAge     int    `yaml:"max_age"`     // days
	Compress   bool   `yaml:"compress"`    // 압축 여부
}

func Load(path string) (*AppConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	bdata, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	cfg := &AppConfig{}
	if err := yaml.Unmarshal(bdata, cfg); err != nil {
		return nil, err
	}

	applyDefaults(cfg)
	resolveRelativePaths(cfg, filepath.Dir(absPath))

	return cfg, nil
}

// resolveRelativePaths resolves relative path fields in the config
// relative to the directory of the config file itself.
// Absolute paths are left unchanged.
func resolveRelativePaths(cfg *AppConfig, base string) {
	resolve := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}

	cfg.Server.CameraDir = resolve(cfg.Server.CameraDir)
	cfg.Server.MvsDir = resolve(cfg.Server.MvsDir)
	cfg.Server.DataDir = resolve(cfg.Server.DataDir)
	cfg.Server.BaseDir = resolve(cfg.Server.BaseDir)
	cfg.FFmpeg.Binary = resolve(cfg.FFmpeg.Binary)
	cfg.FFmpeg.Defaults.ProbeBinary = resolve(cfg.FFmpeg.Defaults.ProbeBinary)
	cfg.Mediamtx.Binary = resolve(cfg.Mediamtx.Binary)
	cfg.Mediamtx.ConfigFile = resolve(cfg.Mediamtx.ConfigFile)
	cfg.AI.Binary = resolve(cfg.AI.Binary)
	cfg.AI.ConfigFile = resolve(cfg.AI.ConfigFile)
	cfg.Log.Dir = resolve(cfg.Log.Dir)
}

func applyDefaults(cfg *AppConfig) {
	cfg.Mediamtx.ApplyDefaults()
}

func validate(cfg *AppConfig) error {
	return nil
}

// LoadRaw reads config.yaml without applying defaults or resolving relative paths.
func LoadRaw(path string) (*AppConfig, error) {
	bdata, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &AppConfig{}
	if err := yaml.Unmarshal(bdata, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes the config to the specified path as YAML.
func Save(path string, cfg *AppConfig) error {
	bdata, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, bdata, 0644)
}
