// Package daemon implements a long-running Unix socket server that accepts
// newline-delimited JSON messages and writes them to the trace store.
// It is an optional optimization: if the daemon is not running, record
// subcommands fall back to direct SQLite writes.
package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// Daemon listens on a Unix domain socket and dispatches newline-delimited JSON
// messages to the trace store.
type Daemon struct {
	socketPath string
	pidPath    string
	dbPath     string
	store      *store.Store
	listener   net.Listener
	mu         sync.Mutex // protects listener field for Stop
}

// New opens the trace store and binds to the Unix socket at socketPath.
// If a stale socket file exists (no daemon responding), it is removed.
// pidPath is derived from socketPath (same dir, file "daemon.pid").
func New(socketPath string, dbPath string) (*Daemon, error) {
	// Remove stale socket if present.
	if _, err := os.Stat(socketPath); err == nil {
		// File exists -- check if anyone is listening.
		c, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
		if err != nil {
			// Stale socket; remove it so we can bind.
			os.Remove(socketPath)
		} else {
			c.Close()
			return nil, fmt.Errorf("daemon already running on %s", socketPath)
		}
	}

	s, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("listen unix %s: %w", socketPath, err)
	}

	pidPath := derivePIDPath(socketPath)

	return &Daemon{
		socketPath: socketPath,
		pidPath:    pidPath,
		dbPath:     dbPath,
		store:      s,
		listener:   ln,
	}, nil
}

// Start begins accepting connections in a background goroutine and writes a
// PID file. It also installs a signal handler for SIGTERM/SIGINT so the
// daemon shuts down cleanly.
func (d *Daemon) Start() error {
	if err := writePID(d.pidPath); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}

	go d.acceptLoop()
	go d.handleSignals()
	return nil
}

// Stop closes the listener and removes the socket and PID files.
func (d *Daemon) Stop() {
	d.mu.Lock()
	ln := d.listener
	d.mu.Unlock()

	if ln != nil {
		ln.Close()
	}
	d.store.Close()
	os.Remove(d.socketPath)
	os.Remove(d.pidPath)
}

// acceptLoop accepts connections and spawns a goroutine for each one.
func (d *Daemon) acceptLoop() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			// listener was closed -- normal shutdown.
			return
		}
		go d.handleConn(conn)
	}
}

// handleSignals waits for SIGTERM or SIGINT and calls Stop.
func (d *Daemon) handleSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch
	d.Stop()
	os.Exit(0)
}

// handleConn reads newline-delimited JSON messages from conn, dispatches each
// to the appropriate handler, and writes a JSON response.
func (d *Daemon) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)

	enc := json.NewEncoder(conn)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg map[string]any
		if err := json.Unmarshal(line, &msg); err != nil {
			enc.Encode(map[string]any{"ok": false, "error": "invalid JSON: " + err.Error()})
			continue
		}

		op, _ := msg["op"].(string)
		var respErr error
		switch op {
		case "ping":
			enc.Encode(map[string]any{"ok": true})
			continue
		case "session_start":
			respErr = d.handleSessionStart(msg)
		case "session_end":
			respErr = d.handleSessionEnd(msg)
		case "prompt":
			respErr = d.handlePrompt(msg)
		case "edit":
			respErr = d.handleEdit(msg)
		case "execution":
			respErr = d.handleExecution(msg)
		default:
			respErr = fmt.Errorf("unknown op: %q", op)
		}

		if respErr != nil {
			enc.Encode(map[string]any{"ok": false, "error": respErr.Error()})
		} else {
			enc.Encode(map[string]any{"ok": true})
		}
	}
}

// --- op handlers ------------------------------------------------------------

func (d *Daemon) handleSessionStart(msg map[string]any) error {
	id, _ := msg["session_id"].(string)
	if id == "" {
		return fmt.Errorf("session_id required")
	}
	cwd, _ := msg["cwd"].(string)
	model_, _ := msg["model"].(string)
	gitHead, _ := msg["git_head"].(string)

	sess := model.Session{
		ID:           id,
		StartedAt:    time.Now().UnixMilli(),
		CWD:          cwd,
		GitHeadStart: gitHead,
		Model:        model_,
	}
	return d.store.InsertSession(sess)
}

func (d *Daemon) handleSessionEnd(msg map[string]any) error {
	id, _ := msg["session_id"].(string)
	if id == "" {
		return fmt.Errorf("session_id required")
	}
	return d.store.EndSession(id, time.Now().UnixMilli(), "")
}

func (d *Daemon) handlePrompt(msg map[string]any) error {
	sessionID, _ := msg["session_id"].(string)
	if sessionID == "" {
		return fmt.Errorf("session_id required")
	}
	content, _ := msg["content"].(string)
	ts := jsonInt64(msg, "timestamp")
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}

	p := model.Prompt{
		SessionID:   sessionID,
		Timestamp:   ts,
		Content:     content,
		ContentHash: util.SHA256HexString(content),
	}
	_, err := d.store.InsertPrompt(p)
	return err
}

func (d *Daemon) handleEdit(msg map[string]any) error {
	sessionID, _ := msg["session_id"].(string)
	if sessionID == "" {
		return fmt.Errorf("session_id required")
	}
	filePath, _ := msg["file_path"].(string)
	tool, _ := msg["tool"].(string)
	diff, _ := msg["diff"].(string)
	ts := jsonInt64(msg, "timestamp")
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}

	lineStart := jsonIntPtr(msg, "line_start")
	lineEnd := jsonIntPtr(msg, "line_end")

	// Resolve latest prompt for the session (best effort).
	latest, _ := d.store.LatestPromptForSession(sessionID)
	var promptID *int64
	if latest != nil {
		promptID = &latest.ID
	}

	e := model.Edit{
		SessionID: sessionID,
		PromptID:  promptID,
		Timestamp: ts,
		FilePath:  filePath,
		Tool:      tool,
		Diff:      diff,
		LineStart: lineStart,
		LineEnd:   lineEnd,
	}
	return d.store.InsertEdit(e)
}

func (d *Daemon) handleExecution(msg map[string]any) error {
	sessionID, _ := msg["session_id"].(string)
	if sessionID == "" {
		return fmt.Errorf("session_id required")
	}
	command, _ := msg["command"].(string)
	classification, _ := msg["classification"].(string)
	ts := jsonInt64(msg, "timestamp")
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}

	exitCodePtr := jsonIntPtr(msg, "exit_code")
	durationPtr := jsonInt64Ptr(msg, "duration_ms")

	x := model.Execution{
		SessionID:      sessionID,
		Timestamp:      ts,
		Command:        command,
		Classification: classification,
		FilesTouched:   "[]",
		ExitCode:       exitCodePtr,
		DurationMS:     durationPtr,
	}
	return d.store.InsertExecution(x)
}

// --- helpers ----------------------------------------------------------------

func derivePIDPath(socketPath string) string {
	// Replace "daemon.sock" with "daemon.pid" in the same directory.
	dir := socketPath
	for i := len(socketPath) - 1; i >= 0; i-- {
		if socketPath[i] == '/' || socketPath[i] == os.PathSeparator {
			dir = socketPath[:i+1]
			break
		}
	}
	return dir + "daemon.pid"
}

func writePID(pidPath string) error {
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

// jsonInt64 extracts a numeric field as int64 from a map[string]any.
// JSON numbers decode as float64, so we cast accordingly.
func jsonInt64(msg map[string]any, key string) int64 {
	v, ok := msg[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}

// jsonInt64Ptr returns a *int64 pointer or nil if the field is absent/zero.
func jsonInt64Ptr(msg map[string]any, key string) *int64 {
	v := jsonInt64(msg, key)
	if v == 0 {
		return nil
	}
	return &v
}

// jsonIntPtr returns a *int pointer or nil if the field is absent/zero.
func jsonIntPtr(msg map[string]any, key string) *int {
	v, ok := msg[key]
	if !ok {
		return nil
	}
	var n int
	switch num := v.(type) {
	case float64:
		n = int(num)
	case int:
		n = num
	default:
		return nil
	}
	return &n
}
