//go:build linux

package server

import (
	"os/exec"
	"syscall"
)

// setPdeathsig sets SIGTERM on the child process when the parent dies.
// This ensures ffmpeg is cleaned up even if neo-blackbox is killed with SIGKILL.
func setPdeathsig(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
}
