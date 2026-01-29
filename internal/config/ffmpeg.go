package config

type ArgKV struct {
	Flag  string `yaml:"flag"`
	Value string `yaml:"value"`
}

type FFmpegConfig struct {
	Binary   string         `yaml:"binary"`
	Defaults FFmpegDefaults `yaml:"defaults"`
	Cameras  []CameraJob    `yaml:"cameras"`
}

type FFmpegDefaults struct {
	InputArgs  []ArgKV `yaml:"input_args"`
	OutputArgs []ArgKV `yaml:"output_args"`
	OutputName string  `yaml:"output_name"`
}

type CameraJob struct {
	ID        string `yaml:"id"`
	RtspURL   string `yaml:"rtsp_url"`
	OutputDIR string `yaml:"output_dir"`

	InputArgs  []ArgKV `yaml:"input_args"`
	MidArgs    []ArgKV `yaml:"mid_args"`
	OutputArgs []ArgKV `yaml:"output_args"`
	OutputName string  `yaml:"output_name"`

	ExtraArgs []ArgKV `yaml:"extra_args"`
}
