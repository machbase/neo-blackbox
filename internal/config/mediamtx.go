package config

type MediamtxConfig struct {
	Binary         string `yaml:"binary"`           // MediaMTX 바이너리 경로 (비어있으면 외부 서버 사용)
	ConfigFile     string `yaml:"config_file"`      // MediaMTX 설정 파일 경로 (비어있으면 바이너리 디렉토리에서 자동 탐색)
	Host           string `yaml:"host"`             // MediaMTX 서버 호스트
	Port           int    `yaml:"port"`             // MediaMTX HTTP API 포트 (기본: 9997)
	WebRTCPort     int    `yaml:"webrtc_port"`      // MediaMTX WebRTC 포트 (기본: 8889)
	RtspServerPort int    `yaml:"rtsp_server_port"` // MediaMTX RTSP 서버 포트 (기본: 8554)
}

// ApplyDefaults sets default values for MediamtxConfig
func (c *MediamtxConfig) ApplyDefaults() {
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 9997
	}
	if c.WebRTCPort == 0 {
		c.WebRTCPort = 8889
	}
	if c.RtspServerPort == 0 {
		c.RtspServerPort = 8554
	}
}
