package plugin_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/plugin"
)

// moduleRoot returns the absolute path to the repository root by walking up
// from this test file's directory until go.mod is found.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine source file path")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod")
		}
		dir = parent
	}
}

// compilePlugin builds the plugin at pkgDir (relative to module root) into a
// temporary binary and returns the binary path and a cleanup function.
func compilePlugin(t *testing.T, pkgDir string) string {
	t.Helper()
	root := moduleRoot(t)
	binPath := filepath.Join(t.TempDir(), "plugin-bin")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", binPath, "./"+pkgDir)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile plugin %s: %v\n%s", pkgDir, err, out)
	}
	return binPath
}

// makeSegment returns a minimal Segment for testing.
func makeSegment(promptText string) analyzer.Segment {
	return analyzer.Segment{
		FilePath:   "foo/bar.go",
		PromptText: promptText,
		EditID:     1,
		SessionID:  "sess-1",
	}
}

// TestPlugin_RunsExecutable compiles the no-prod-secrets plugin, runs it with
// a fixture segment containing an AWS key, and verifies the expected signal is
// returned.
func TestPlugin_RunsExecutable(t *testing.T) {
	binPath := compilePlugin(t, "plugins/no-prod-secrets")

	p := plugin.Plugin{Name: "no-prod-secrets", Path: binPath}
	seg := makeSegment("diff --git a/foo.go b/foo.go\nAKIAIOSFODNN7EXAMPLE plus extra chars AKIAIOSFODNN7EXAM")
	// Build a 20-char AWS key that matches the regex AKIA[0-9A-Z]{16}
	seg.PromptText = "aws_access_key_id = AKIAIOSFODNN7EXAMPLE"

	result, err := p.Run(context.Background(), seg)
	if err != nil {
		t.Fatalf("plugin.Run returned error: %v", err)
	}
	if len(result.Signals) == 0 {
		t.Fatal("expected at least one signal, got none")
	}
	found := false
	for _, s := range result.Signals {
		if s.Code == "plugin:NO_PROD_SECRETS" {
			found = true
			if s.Tier != 1 {
				t.Errorf("expected Tier 1, got %d", s.Tier)
			}
		}
	}
	if !found {
		t.Errorf("signal plugin:NO_PROD_SECRETS not found; got %+v", result.Signals)
	}
}

// slowPluginSrc is a minimal Go program that sleeps 10 seconds before
// responding. It is compiled into a binary for the timeout test so that the
// process is killed directly by exec.CommandContext (no intermediate shell).
const slowPluginSrc = `package main

import (
	"fmt"
	"time"
)

func main() {
	time.Sleep(10 * time.Second)
	fmt.Println(` + "`" + `{"signals":[],"error":null}` + "`" + `)
}
`

// TestPlugin_Timeout verifies that a plugin that sleeps 10 seconds is killed
// after the 5-second timeout and returns an empty result (no panic).
func TestPlugin_Timeout(t *testing.T) {
	// Build a Go binary that sleeps 10 seconds so exec.CommandContext kills
	// the exact process (no intermediate shell escaping the signal).
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "slow.go")
	if err := os.WriteFile(srcPath, []byte(slowPluginSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, "slow-plugin")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", binPath, srcPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compile slow plugin: %v\n%s", err, out)
	}

	p := plugin.Plugin{Name: "slow", Path: binPath}

	start := time.Now()
	result, err := p.Run(context.Background(), makeSegment("hello"))
	elapsed := time.Since(start)

	// We expect an error (timeout) and an empty result.
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if len(result.Signals) != 0 {
		t.Errorf("expected empty signals on timeout, got %+v", result.Signals)
	}
	// Timeout should fire well under 7 seconds.
	if elapsed > 7*time.Second {
		t.Errorf("plugin took too long: %v (expected <= 7s)", elapsed)
	}
}

// badPluginSrc is a minimal Go program that writes invalid JSON to stdout.
const badPluginSrc = `package main

import "fmt"

func main() {
	fmt.Println("NOT_VALID_JSON")
}
`

// TestPlugin_ErrorReturnsEmpty verifies that a plugin writing invalid JSON
// causes RunAll to return an empty slice without panicking.
func TestPlugin_ErrorReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(srcPath, []byte(badPluginSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, "bad-plugin")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", binPath, srcPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compile bad plugin: %v\n%s", err, out)
	}

	plugins := []plugin.Plugin{{Name: "bad", Path: binPath}}
	signals := plugin.RunAll(context.Background(), plugins, makeSegment("hello"))
	if signals == nil {
		signals = []plugin.Signal{}
	}
	if len(signals) != 0 {
		t.Errorf("expected empty signals, got %+v", signals)
	}
}

// makePrintPlugin compiles a tiny Go binary that prints a fixed JSON line.
func makePrintPlugin(t *testing.T, dir, name, jsonLine string) string {
	t.Helper()
	src := fmt.Sprintf(`package main

import "fmt"

func main() {
	fmt.Println(%q)
}
`, jsonLine)
	srcPath := filepath.Join(dir, name+".go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", binPath, srcPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compile %s plugin: %v\n%s", name, err, out)
	}
	return binPath
}

// TestRunAll_MergesSignals verifies that two plugins each returning one signal
// produce a merged slice with 2 signals.
func TestRunAll_MergesSignals(t *testing.T) {
	dir := t.TempDir()

	sig1JSON := `{"signals":[{"code":"plugin:SIG_ONE","tier":2,"message":"first"}],"error":null}`
	sig2JSON := `{"signals":[{"code":"plugin:SIG_TWO","tier":3,"message":"second"}],"error":null}`

	plugins := []plugin.Plugin{
		{Name: "p1", Path: makePrintPlugin(t, dir, "p1", sig1JSON)},
		{Name: "p2", Path: makePrintPlugin(t, dir, "p2", sig2JSON)},
	}

	signals := plugin.RunAll(context.Background(), plugins, makeSegment("hello"))
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d: %+v", len(signals), signals)
	}
	codes := make(map[string]bool)
	for _, s := range signals {
		codes[s.Code] = true
	}
	if !codes["plugin:SIG_ONE"] {
		t.Error("missing signal plugin:SIG_ONE")
	}
	if !codes["plugin:SIG_TWO"] {
		t.Error("missing signal plugin:SIG_TWO")
	}
}
