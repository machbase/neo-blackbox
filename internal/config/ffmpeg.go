package config

type FFmpegConfig struct {
	Binary   string         `yaml:"binary"`
	Defaults FFmpegDefaults `yaml:"defaults"`
	Cameras  []CameraJob    `yaml:"cameras"`
}

type FFmpegDefaults struct {
	InputArgs  []string `yaml:"input_args"`
	OutputArgs []string `yaml:"output_args"`
	OutputName string   `yaml:"output_name"`
}

type CameraJob struct {
	ID        string `yaml:"id"`
	RtspURL   string `yaml:"rtsp_url"`
	OutputDIR string `yaml:"output_dir"`

	InputArgs  []string `yaml:"input_args"`
	OutputArgs []string `yaml:"output_args"`
	OutputName string   `yaml:"output_name"`

	ExtraArgs []string `yaml:"extra_args"`
}
