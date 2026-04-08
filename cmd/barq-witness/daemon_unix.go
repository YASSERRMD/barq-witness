//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detachProcess sets the Setsid flag so the daemon child process starts
// a new session and outlives the parent process group.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
