//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func setPGIDUnix(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killUnix(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
