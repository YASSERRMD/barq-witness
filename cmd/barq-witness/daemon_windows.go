//go:build windows

package main

import "os/exec"

// detachProcess is a no-op on Windows. Unix domain sockets are supported
// on Windows 10 1803+ so the daemon itself works, but Setsid does not exist
// in syscall.SysProcAttr on Windows.
func detachProcess(cmd *exec.Cmd) {}
