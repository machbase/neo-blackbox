package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type PingRequest struct {
	IP      string `json:"ip" binding:"required"`
	Timeout int    `json:"timeout"` // seconds, max 5
}

type PingResult struct {
	IP      string  `json:"ip"`
	Alive   bool    `json:"alive"`
	Latency string  `json:"latency,omitempty"`
	Output  string  `json:"output,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// PingIP handles POST /api/cameras/ping
// Request: { "ip": "192.168.1.1", "timeout": 3 }
// timeout 기본값 5초, 최대 5초
func (h *Handler) PingIP(c *gin.Context) {
	tick := time.Now()

	var req PingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, tick, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ip := strings.TrimSpace(req.IP)

	// IP 유효성 검사 (command injection 방지)
	if net.ParseIP(ip) == nil {
		errorResponse(c, tick, http.StatusBadRequest, fmt.Sprintf("invalid ip address: %q", ip))
		return
	}

	timeout := req.Timeout
	if timeout <= 0 || timeout > 5 {
		timeout = 5
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeout)*time.Second)
	defer cancel()

	start := time.Now()
	// -c 1: 1회, -W: 대기시간(초)
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", fmt.Sprintf("%d", timeout), ip)
	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	result := PingResult{
		IP:     ip,
		Output: strings.TrimSpace(string(output)),
	}

	if err != nil {
		result.Alive = false
		result.Error = err.Error()
	} else {
		result.Alive = true
		result.Latency = elapsed.String()
	}

	successResponse(c, tick, result)
}
