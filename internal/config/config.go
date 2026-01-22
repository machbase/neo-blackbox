package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	FFmpeg  FFmpegConfig  `yaml:"ffmpeg"`
	Watcher WatcherConfig `yaml:"watcher"`
	Server  ServerConfig  `yaml:"server"`
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
