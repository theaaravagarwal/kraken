//go:build windows

package main

import "os/exec"

func setPGIDUnix(cmd *exec.Cmd) {
	// No-op on Windows
}

func killUnix(pid int) error {
	// This path should not be reached on Windows due to isWindows() check in caller
	panic("killUnix not supported on Windows")
}
