package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	FFmpeg   FFmpegConfig   `yaml:"ffmpeg"`
	Server   ServerConfig   `yaml:"server"`
	Machbase MachbaseConfig `yaml:"machbase"`
	Log      LogConfig      `yaml:"log"`
}

type LogConfig struct {
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
	bdata, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &AppConfig{}
	if err := yaml.Unmarshal(bdata, cfg); err != nil {
		return nil, err
	}

	applyDefaults(cfg)

	return cfg, nil
}

func applyDefaults(cfg *AppConfig) {

}

func validate(cfg *AppConfig) error {
	return nil
}
