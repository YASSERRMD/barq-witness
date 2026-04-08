package main

// daemon.go implements the `barq-witness daemon` subcommands:
//
//   barq-witness daemon start   -- starts daemon in background
//   barq-witness daemon stop    -- sends SIGTERM to the daemon PID
//   barq-witness daemon status  -- prints running state
//
// The hidden flag --daemon-foreground is used internally when the binary
// re-execs itself to become the actual server process.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/yasserrmd/barq-witness/internal/daemon"
)

// detachProcess is implemented in daemon_unix.go (!windows) and daemon_windows.go.
// On Unix it sets Setsid so the child outlives the parent process group.
// On Windows it is a no-op.

// runDaemonForeground is called when the binary detects --daemon-foreground.
// It starts the daemon server and blocks until the process receives a signal.
func runDaemonForeground(witnessDir string) {
	socketPath := filepath.Join(witnessDir, "daemon.sock")
	dbPath := filepath.Join(witnessDir, "trace.db")

	d, err := daemon.New(socketPath, dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: %v\n", err)
		os.Exit(1)
	}
	if err := d.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon start: %v\n", err)
		os.Exit(1)
	}
	// Block forever -- the daemon's signal handler calls os.Exit(0) on SIGTERM.
	select {}
}

// runDaemon dispatches daemon subcommands.
func runDaemon(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: barq-witness daemon <start|stop|status>")
		os.Exit(1)
	}

	witnessDir := resolveWitnessDir()
	socketPath := filepath.Join(witnessDir, "daemon.sock")
	pidPath := filepath.Join(witnessDir, "daemon.pid")

	switch args[0] {
	case "start":
		daemonStart(witnessDir, socketPath, pidPath)
	case "stop":
		daemonStop(pidPath)
	case "status":
		daemonStatus(socketPath, pidPath)
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

// daemonStart forks the current binary with --daemon-foreground and detaches.
func daemonStart(witnessDir, socketPath, pidPath string) {
	// Check if already running.
	if daemon.IsDaemonRunning(socketPath) {
		pid := readPIDFile(pidPath)
		fmt.Printf("daemon already running (pid %s)\n", pid)
		return
	}

	// Ensure the .witness directory exists.
	if err := os.MkdirAll(witnessDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "daemon start: mkdir: %v\n", err)
		os.Exit(1)
	}

	// Re-exec self with hidden flag.
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon start: executable: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, "--daemon-foreground", "--witness-dir="+witnessDir)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// Detach from the parent process group so the child outlives us.
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon start: exec: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("daemon started (pid %d)\n", cmd.Process.Pid)
}

// daemonStop sends SIGTERM to the PID stored in the PID file.
func daemonStop(pidPath string) {
	pid, err := readPIDFileInt(pidPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "daemon not running (no pid file)")
		os.Exit(1)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon stop: find process: %v\n", err)
		os.Exit(1)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "daemon stop: signal: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("daemon stopped (pid %d)\n", pid)
}

// daemonStatus prints whether the daemon is running.
func daemonStatus(socketPath, pidPath string) {
	if daemon.IsDaemonRunning(socketPath) {
		pid := readPIDFile(pidPath)
		fmt.Printf("running (pid %s)\n", pid)
	} else {
		fmt.Println("not running")
	}
}

// readPIDFile returns the content of the PID file as a string, or "unknown".
func readPIDFile(pidPath string) string {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

// readPIDFileInt reads the PID file and parses it as an integer.
func readPIDFileInt(pidPath string) (int, error) {
	s := readPIDFile(pidPath)
	if s == "unknown" {
		return 0, fmt.Errorf("no pid file")
	}
	return strconv.Atoi(s)
}
