package main

// record.go implements the five `barq-witness record <event>` subcommands
// that Claude Code hook scripts invoke.  Every subcommand MUST exit 0 even
// on internal errors; errors are appended to .witness/barq-witness.log.
//
// Daemon fallback: if a daemon is running on .witness/daemon.sock, events are
// forwarded to it via the Unix socket instead of opening SQLite directly.
// If the daemon is not running, the existing direct SQLite write is used.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/yasserrmd/barq-witness/internal/daemon"
	"github.com/yasserrmd/barq-witness/internal/hooks"
	"github.com/yasserrmd/barq-witness/internal/model"
	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// runRecord dispatches to the correct record subcommand.
// It never returns a non-zero exit; all errors are logged.
func runRecord(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: barq-witness record <session-start|prompt|edit|exec|session-end>")
		os.Exit(1)
	}

	witnessDir := resolveWitnessDir()
	if err := os.MkdirAll(witnessDir, 0o755); err != nil {
		// Can't even create the directory -- bail silently.
		os.Exit(0)
	}

	logger := util.OpenLogger(witnessDir)
	dbPath := filepath.Join(witnessDir, "trace.db")

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		logger.Printf("record %s: read stdin: %v", args[0], err)
		os.Exit(0)
	}

	switch args[0] {
	case "session-start":
		recordSessionStart(data, dbPath, witnessDir, logger)
	case "prompt":
		recordPrompt(data, dbPath, logger)
	case "edit":
		recordEdit(data, dbPath, logger)
	case "exec":
		recordExec(data, dbPath, logger)
	case "session-end":
		recordSessionEnd(data, dbPath, witnessDir, logger)
	default:
		fmt.Fprintf(os.Stderr, "unknown record subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

// resolveWitnessDir returns the .witness directory path.
// Prefers $CLAUDE_PROJECT_DIR if set, otherwise the current working directory.
func resolveWitnessDir() string {
	if base := os.Getenv("CLAUDE_PROJECT_DIR"); base != "" {
		return filepath.Join(base, ".witness")
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".witness")
}

// openStore opens the trace store, logging and exiting 0 on failure.
func openStore(dbPath string, logger interface{ Printf(string, ...any) }) *store.Store {
	s, err := store.Open(dbPath)
	if err != nil {
		logger.Printf("open store: %v", err)
		os.Exit(0)
	}
	return s
}

// dialDaemon attempts to connect to a running daemon at the socket in
// witnessDir. Returns nil (not an error) if the daemon is not running --
// callers should fall back to direct SQLite writes in that case.
func dialDaemon(witnessDir string) *daemon.Client {
	socketPath := filepath.Join(witnessDir, "daemon.sock")
	c, err := daemon.Dial(socketPath)
	if err != nil {
		return nil
	}
	if !c.Ping() {
		c.Close()
		return nil
	}
	return c
}

// sendToDaemon sends a message via the client and returns true on success.
func sendToDaemon(c *daemon.Client, msg map[string]any, logger interface{ Printf(string, ...any) }, op string) bool {
	resp, err := c.Send(msg)
	if err != nil {
		logger.Printf("%s: daemon send: %v", op, err)
		return false
	}
	ok, _ := resp["ok"].(bool)
	if !ok {
		errMsg, _ := resp["error"].(string)
		logger.Printf("%s: daemon error: %s", op, errMsg)
		return false
	}
	return true
}

// ---- session-start ----------------------------------------------------------

func recordSessionStart(data []byte, dbPath, witnessDir string, logger interface{ Printf(string, ...any) }) {
	p, err := hooks.ParseSessionStart(data)
	if err != nil {
		logger.Printf("session-start: parse: %v", err)
		os.Exit(0)
	}

	cwd := p.CWD
	if cwd == "" {
		cwd = witnessDir[:len(witnessDir)-len("/.witness")]
	}

	head, _ := util.HeadSHA(cwd)

	// Try daemon first.
	if c := dialDaemon(witnessDir); c != nil {
		defer c.Close()
		msg := map[string]any{
			"op":         "session_start",
			"session_id": p.SessionID,
			"cwd":        cwd,
			"model":      p.Model,
			"git_head":   head,
		}
		if sendToDaemon(c, msg, logger, "session-start") {
			return
		}
	}

	s := openStore(dbPath, logger)
	defer s.Close()

	sess := model.Session{
		ID:           p.SessionID,
		StartedAt:    time.Now().UnixMilli(),
		CWD:          cwd,
		GitHeadStart: head,
		Model:        p.Model,
	}
	if err := s.InsertSession(sess); err != nil {
		logger.Printf("session-start: insert: %v", err)
	}
}

// ---- prompt -----------------------------------------------------------------

func recordPrompt(data []byte, dbPath string, logger interface{ Printf(string, ...any) }) {
	p, err := hooks.ParseUserPrompt(data)
	if err != nil {
		logger.Printf("prompt: parse: %v", err)
		os.Exit(0)
	}

	witnessDir := filepath.Dir(dbPath)
	now := time.Now().UnixMilli()

	// Try daemon first.
	if c := dialDaemon(witnessDir); c != nil {
		defer c.Close()
		msg := map[string]any{
			"op":         "prompt",
			"session_id": p.SessionID,
			"content":    p.Prompt,
			"timestamp":  now,
		}
		if sendToDaemon(c, msg, logger, "prompt") {
			return
		}
	}

	s := openStore(dbPath, logger)
	defer s.Close()

	prompt := model.Prompt{
		SessionID:   p.SessionID,
		Timestamp:   now,
		Content:     p.Prompt,
		ContentHash: util.SHA256HexString(p.Prompt),
	}
	if _, err := s.InsertPrompt(prompt); err != nil {
		logger.Printf("prompt: insert: %v", err)
	}
}

// ---- edit -------------------------------------------------------------------

func recordEdit(data []byte, dbPath string, logger interface{ Printf(string, ...any) }) {
	p, err := hooks.ParsePostToolUse(data)
	if err != nil {
		logger.Printf("edit: parse: %v", err)
		os.Exit(0)
	}

	var before, after string
	switch p.ToolName {
	case "Edit":
		before = p.ToolInput.OldString
		after = p.ToolInput.NewString
	case "MultiEdit":
		// Concatenate all old strings as "before" and new strings as "after"
		// for a simplified diff; the individual diffs are best-effort.
		var bParts, aParts []string
		for _, e := range p.ToolInput.Edits {
			bParts = append(bParts, e.OldString)
			aParts = append(aParts, e.NewString)
		}
		before = strings.Join(bParts, "\n")
		after = strings.Join(aParts, "\n")
	case "Write":
		before = readFileIfExists(p.ToolInput.FilePath)
		after = p.ToolInput.Content
	}

	diffStr := computeUnifiedDiff(before, after)
	lineStart, lineEnd := computeLineRange(p.ToolInput.FilePath, p.ToolInput.OldString, before, after)
	now := time.Now().UnixMilli()

	witnessDir := filepath.Dir(dbPath)

	// Try daemon first.
	if c := dialDaemon(witnessDir); c != nil {
		defer c.Close()
		msg := map[string]any{
			"op":         "edit",
			"session_id": p.SessionID,
			"file_path":  p.ToolInput.FilePath,
			"tool":       p.ToolName,
			"diff":       diffStr,
			"timestamp":  now,
		}
		if lineStart != nil {
			msg["line_start"] = *lineStart
		}
		if lineEnd != nil {
			msg["line_end"] = *lineEnd
		}
		if sendToDaemon(c, msg, logger, "edit") {
			return
		}
	}

	s := openStore(dbPath, logger)
	defer s.Close()

	latest, err := s.LatestPromptForSession(p.SessionID)
	if err != nil {
		logger.Printf("edit: latest prompt: %v", err)
	}
	var promptID *int64
	if latest != nil {
		promptID = &latest.ID
	}

	edit := model.Edit{
		SessionID:  p.SessionID,
		PromptID:   promptID,
		Timestamp:  now,
		FilePath:   p.ToolInput.FilePath,
		Tool:       p.ToolName,
		BeforeHash: util.SHA256HexString(before),
		AfterHash:  util.SHA256HexString(after),
		LineStart:  lineStart,
		LineEnd:    lineEnd,
		Diff:       diffStr,
	}
	if err := s.InsertEdit(edit); err != nil {
		logger.Printf("edit: insert: %v", err)
	}
}

// ---- exec -------------------------------------------------------------------

func recordExec(data []byte, dbPath string, logger interface{ Printf(string, ...any) }) {
	p, err := hooks.ParsePostToolUse(data)
	if err != nil {
		logger.Printf("exec: parse: %v", err)
		os.Exit(0)
	}

	cmd := p.ToolInput.Command
	class := classifyCommand(cmd)
	touched := extractFilesTouched(cmd)
	touchedJSON, _ := json.Marshal(touched)
	now := time.Now().UnixMilli()

	witnessDir := filepath.Dir(dbPath)

	// Try daemon first.
	if c := dialDaemon(witnessDir); c != nil {
		defer c.Close()
		msg := map[string]any{
			"op":             "execution",
			"session_id":     p.SessionID,
			"command":        cmd,
			"classification": class,
			"timestamp":      now,
		}
		if p.ToolResponse.ExitCode != nil {
			msg["exit_code"] = *p.ToolResponse.ExitCode
		}
		if p.ToolResponse.DurationMS != nil {
			msg["duration_ms"] = *p.ToolResponse.DurationMS
		}
		if sendToDaemon(c, msg, logger, "exec") {
			return
		}
	}

	x := model.Execution{
		SessionID:      p.SessionID,
		Timestamp:      now,
		Command:        cmd,
		Classification: class,
		FilesTouched:   string(touchedJSON),
		ExitCode:       p.ToolResponse.ExitCode,
		DurationMS:     p.ToolResponse.DurationMS,
	}

	s := openStore(dbPath, logger)
	defer s.Close()

	if err := s.InsertExecution(x); err != nil {
		logger.Printf("exec: insert: %v", err)
	}
}

// ---- session-end ------------------------------------------------------------

func recordSessionEnd(data []byte, dbPath, witnessDir string, logger interface{ Printf(string, ...any) }) {
	p, err := hooks.ParseSessionEnd(data)
	if err != nil {
		logger.Printf("session-end: parse: %v", err)
		os.Exit(0)
	}

	cwd := p.CWD
	if cwd == "" {
		cwd = witnessDir[:len(witnessDir)-len("/.witness")]
	}

	head, _ := util.HeadSHA(cwd)

	// Try daemon first.
	if c := dialDaemon(witnessDir); c != nil {
		defer c.Close()
		msg := map[string]any{
			"op":         "session_end",
			"session_id": p.SessionID,
		}
		if sendToDaemon(c, msg, logger, "session-end") {
			return
		}
	}

	s := openStore(dbPath, logger)
	defer s.Close()

	if err := s.EndSession(p.SessionID, time.Now().UnixMilli(), head); err != nil {
		logger.Printf("session-end: update: %v", err)
	}
}

// ============================================================
// Helpers
// ============================================================

// classifyCommand maps a shell command to one of the five categories.
func classifyCommand(cmd string) string {
	lower := strings.ToLower(cmd)
	trimmed := strings.TrimSpace(lower)

	testKeywords := []string{
		"go test", "pytest", "npm test", "yarn test", "cargo test",
		"jest", "vitest", "mocha", "rspec",
	}
	for _, kw := range testKeywords {
		if strings.Contains(lower, kw) {
			return "test"
		}
	}

	if strings.HasPrefix(trimmed, "git ") {
		return "git"
	}

	installKeywords := []string{
		"go get", "pip install", "npm install", "yarn add",
		"cargo add", "apt ", "brew install",
	}
	for _, kw := range installKeywords {
		if strings.Contains(lower, kw) {
			return "install"
		}
	}

	runKeywords := []string{
		"go run", "python ", "node ", "./", "cargo run", "npm start", "yarn dev",
	}
	for _, kw := range runKeywords {
		if strings.Contains(lower, kw) {
			return "run"
		}
	}

	return "other"
}

// fileArgPattern matches tokens that look like file paths (contain a dot
// and at least one path separator OR start with ./).
var fileArgPattern = regexp.MustCompile(
	`(?:^|\s)((?:\./|/|[a-zA-Z0-9_\-]+/)?\S+\.[a-zA-Z0-9]{1,10})(?:\s|$)`,
)

// extractFilesTouched does a best-effort parse of a shell command for
// file-path arguments.  Returns nil (not an empty slice) if none found.
func extractFilesTouched(cmd string) []string {
	matches := fileArgPattern.FindAllStringSubmatch(cmd, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, m := range matches {
		f := strings.TrimSpace(m[1])
		if f == "" || seen[f] {
			continue
		}
		// Reject obvious non-paths (flags, URLs).
		if strings.HasPrefix(f, "-") || strings.Contains(f, "://") {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// computeUnifiedDiff produces a patch-format diff string from two text blobs.
func computeUnifiedDiff(before, after string) string {
	if before == after {
		return ""
	}
	dmp := diffmatchpatch.New()
	wca, wcb, wc := dmp.DiffLinesToChars(before, after)
	diffs := dmp.DiffMain(wca, wcb, false)
	diffs = dmp.DiffCharsToLines(diffs, wc)
	diffs = dmp.DiffCleanupSemantic(diffs)
	patches := dmp.PatchMake(before, diffs)
	return dmp.PatchToText(patches)
}

// computeLineRange returns best-effort line_start and line_end for an edit.
// It attempts to locate oldString in the file on disk to determine position.
func computeLineRange(filePath, oldString, before, after string) (*int, *int) {
	if filePath == "" {
		return nil, nil
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		// For Write (new file), line range spans the whole output.
		if before == "" && after != "" {
			start := 1
			end := strings.Count(after, "\n") + 1
			if !strings.HasSuffix(after, "\n") {
				end++
			}
			return &start, &end
		}
		return nil, nil
	}

	fileContent := string(fileBytes)
	if oldString == "" {
		// Write to existing file -- treat as full replacement.
		start := 1
		end := strings.Count(fileContent, "\n") + 1
		return &start, &end
	}

	idx := strings.Index(fileContent, oldString)
	if idx < 0 {
		return nil, nil
	}

	linesBefore := strings.Count(fileContent[:idx], "\n")
	start := linesBefore + 1
	end := start + strings.Count(oldString, "\n")
	return &start, &end
}

// readFileIfExists returns the content of path, or "" if unreadable.
func readFileIfExists(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if !utf8.Valid(b) {
		return ""
	}
	return string(b)
}
