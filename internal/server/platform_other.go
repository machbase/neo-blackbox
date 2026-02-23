//go:build !linux

package server

import "os/exec"

func setPdeathsig(cmd *exec.Cmd) {}
