package config

type WatcherConfig struct {
	Rules []WatcherRule `yaml:"rules"`
}

type WatcherRule struct {
	SourceDir string `yaml:"source_dir"`
	TargetDir string `yaml:"target_dir"`
	Ext       string `yaml:"ext"`
}
