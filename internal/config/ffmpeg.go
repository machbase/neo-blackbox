package config

type ArgKV struct {
	Flag  string `yaml:"flag"`
	Value string `yaml:"value"`
}

type FFmpegConfig struct {
	Binary   string         `yaml:"binary"`
	Defaults FFmpegDefaults `yaml:"defaults"`
}

type FFmpegDefaults struct {
	ProbeBinary string  `yaml:"probe_binary"`
	ProbeArgs   []ArgKV `yaml:"probe_args"`
}
