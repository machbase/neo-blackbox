package mediamtx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"neo-blackbox/internal/logger"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Config는 MediaMTX 서버 설정
type Config struct {
	Binary     string   // MediaMTX 실행 파일 경로 (로컬 서버용)
	ConfigFile string   // 설정 파일 경로
	ServerURL  string   // 외부 서버 URL (예: rtsp://localhost:8554)
	Args       []string // 추가 실행 인자
}

// Runner는 MediaMTX 미디어 서버 관리
type Runner struct {
	cfg    Config
	cmd    *exec.Cmd
	mu     sync.Mutex
	cancel context.CancelFunc
}

// ServerStatus는 서버 상태 정보
type ServerStatus struct {
	Running   bool
	PID       int
	StartedAt time.Time
	Uptime    time.Duration
}

// New는 새로운 MediaMTX Runner 생성
func New(cfg Config) *Runner {
	return &Runner{
		cfg: cfg,
	}
}

// Start는 로컬 MediaMTX 서버 시작
func (r *Runner) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		return fmt.Errorf("server already running (PID: %d)", r.cmd.Process.Pid)
	}

	execArgs := r.buildExecArgs()
	logger.GetLogger().Infof("Starting MediaMTX server: %s", prettyCommand(r.cfg.Binary, execArgs))

	cmdCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	cmd := exec.CommandContext(cmdCtx, r.cfg.Binary, execArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start MediaMTX: %w", err)
	}

	r.cmd = cmd
	logger.GetLogger().Infof("MediaMTX server started (PID: %d)", cmd.Process.Pid)

	// 백그라운드에서 프로세스 종료 대기
	go func() {
		err := cmd.Wait()
		r.mu.Lock()
		r.cmd = nil
		r.mu.Unlock()

		if err != nil {
			logger.GetLogger().Errorf("MediaMTX server exited with error: %v", err)
		} else {
			logger.GetLogger().Info("MediaMTX server stopped")
		}
	}()

	return nil
}

// Stop은 실행 중인 MediaMTX 서버 중지
func (r *Runner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return fmt.Errorf("server not running")
	}

	pid := r.cmd.Process.Pid
	logger.GetLogger().Infof("Stopping MediaMTX server (PID: %d)", pid)

	if r.cancel != nil {
		r.cancel()
	}

	// 프로세스 종료 대기 (최대 5초)
	done := make(chan error, 1)
	go func() {
		done <- r.cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		// 타임아웃 시 강제 종료
		if err := r.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		logger.GetLogger().Warn("MediaMTX server forcefully killed")
	case err := <-done:
		if err != nil && err.Error() != "signal: killed" {
			logger.GetLogger().Debugf("MediaMTX server stop error: %v", err)
		}
	}

	r.cmd = nil
	logger.GetLogger().Info("MediaMTX server stopped successfully")
	return nil
}

// Status는 현재 서버 상태 반환
func (r *Runner) Status() ServerStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	status := ServerStatus{
		Running: false,
	}

	if r.cmd != nil && r.cmd.Process != nil {
		status.Running = true
		status.PID = r.cmd.Process.Pid
		// TODO: 시작 시간 추적 필요 시 구현
	}

	return status
}

// TestConnection은 MediaMTX 서버 연결 테스트
// 외부 서버 또는 로컬 서버가 응답하는지 확인
func (r *Runner) TestConnection(ctx context.Context) error {
	if r.cfg.ServerURL == "" {
		return fmt.Errorf("server URL not configured")
	}

	// TODO: 실제 연결 테스트 구현
	// rtsp://, http://, ws:// 등 프로토콜에 따라 다른 방식으로 테스트
	logger.GetLogger().Debugf("Testing connection to MediaMTX server: %s", r.cfg.ServerURL)

	// 간단한 ping 테스트 예시 (실제로는 프로토콜에 맞는 테스트 필요)
	// 예: RTSP OPTIONS 요청, HTTP GET 요청 등

	return fmt.Errorf("connection test not implemented yet")
}

// GetServerURL은 설정된 서버 URL 반환
func (r *Runner) GetServerURL() string {
	return r.cfg.ServerURL
}

// buildExecArgs는 실행 인자 생성
func (r *Runner) buildExecArgs() []string {
	args := []string{}

	// 설정 파일이 있으면 추가
	if r.cfg.ConfigFile != "" {
		args = append(args, r.cfg.ConfigFile)
	}

	// 추가 인자 추가
	args = append(args, r.cfg.Args...)

	return args
}

// prettyCommand는 명령어를 보기 좋게 포맷팅
func prettyCommand(binary string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(binary))

	for _, arg := range args {
		parts = append(parts, " "+shellQuote(arg))
	}

	return strings.Join(parts, "")
}

// shellQuote는 쉘 명령어 인자를 따옴표로 감싸기
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}

	needQuote := false
	for _, c := range s {
		if c == ' ' || c == '\n' || c == '\t' || c == '\'' || c == '"' || c == '\\' {
			needQuote = true
			break
		}
	}

	if !needQuote {
		return s
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ProbeServer는 서버 정보 조회 (HTTP API 사용)
// MediaMTX는 HTTP API를 제공하므로 이를 통해 서버 정보 조회 가능
func (r *Runner) ProbeServer(ctx context.Context) (map[string]interface{}, error) {
	// TODO: MediaMTX HTTP API 호출 구현
	// GET /v3/config, /v3/paths 등
	return nil, fmt.Errorf("probe server not implemented yet")
}

// RunCommand는 일회성 명령 실행 (ffmpeg의 Probe 같은 용도)
func (r *Runner) RunCommand(ctx context.Context, args ...string) (string, error) {
	if r.cfg.Binary == "" {
		return "", fmt.Errorf("binary path not configured")
	}

	logger.GetLogger().Debugf("Running command: %s", prettyCommand(r.cfg.Binary, args))

	var outBuf, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, r.cfg.Binary, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %w, stderr: %s", err, errBuf.String())
	}

	return outBuf.String(), nil
}

// HeartbeatResponse represents MediaMTX API response
type HeartbeatResponse struct {
	Status  string                 `json:"status"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Message string                 `json:"message,omitempty"`
}

// Heartbeat checks if MediaMTX server is alive using HTTP API
// mediaURL format: http://host:port or rtsp://host:port (will convert to HTTP API)
func Heartbeat(ctx context.Context, mediaURL string, timeout time.Duration) (*HeartbeatResponse, error) {
	if mediaURL == "" {
		return nil, fmt.Errorf("media URL is empty")
	}

	// Parse URL and convert to HTTP API endpoint
	parsedURL, err := url.Parse(mediaURL)
	if err != nil {
		return nil, fmt.Errorf("invalid media URL: %w", err)
	}

	// Default MediaMTX HTTP API port is 9997
	apiURL := fmt.Sprintf("http://%s:9997/v3/config/global/get", parsedURL.Hostname())

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &HeartbeatResponse{
			Status:  "error",
			Message: fmt.Sprintf("connection failed: %v", err),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HeartbeatResponse{
			Status:  "unhealthy",
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		}, fmt.Errorf("unhealthy: status code %d", resp.StatusCode)
	}

	// Try to parse response
	var apiResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		// If parsing fails, still consider it healthy (server responded)
		return &HeartbeatResponse{
			Status:  "healthy",
			Message: "server responded",
		}, nil
	}

	return &HeartbeatResponse{
		Status: "healthy",
		Data:   apiResp,
	}, nil
}
