package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/config"
)

func TestDefault_HasSafeValues(t *testing.T) {
	cfg := config.Default()
	if cfg.Explainer.Backend != "null" {
		t.Errorf("default backend = %q, want null", cfg.Explainer.Backend)
	}
	if cfg.Explainer.TimeoutMS != 5000 {
		t.Errorf("default timeout_ms = %d, want 5000", cfg.Explainer.TimeoutMS)
	}
	if cfg.Analyzer.IntentMatchThreshold != 0.5 {
		t.Errorf("default intent_match_threshold = %v, want 0.5", cfg.Analyzer.IntentMatchThreshold)
	}
	if cfg.Privacy.Mode {
		t.Error("default privacy mode should be false")
	}
}

func TestLoad_MissingFile_ReturnsDefault(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if cfg.Explainer.Backend != "null" {
		t.Errorf("expected null backend, got %q", cfg.Explainer.Backend)
	}
}

func TestLoad_ValidTOML(t *testing.T) {
	dir := t.TempDir()
	content := `
[explainer]
backend    = "claude"
model      = "claude-haiku-4-5"
timeout_ms = 3000

[analyzer]
security_paths_extra = ["**/legacy/**"]
exclude_paths = ["vendor/**"]

[privacy]
mode = true
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Explainer.Backend != "claude" {
		t.Errorf("backend = %q, want claude", cfg.Explainer.Backend)
	}
	if cfg.Explainer.Model != "claude-haiku-4-5" {
		t.Errorf("model = %q", cfg.Explainer.Model)
	}
	if cfg.Explainer.TimeoutMS != 3000 {
		t.Errorf("timeout_ms = %d, want 3000", cfg.Explainer.TimeoutMS)
	}
	if len(cfg.Analyzer.SecurityPathsExtra) != 1 || cfg.Analyzer.SecurityPathsExtra[0] != "**/legacy/**" {
		t.Errorf("security_paths_extra = %v", cfg.Analyzer.SecurityPathsExtra)
	}
	if !cfg.Privacy.Mode {
		t.Error("privacy.mode should be true")
	}
}

func TestLoad_InvalidTOML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[[[bad toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoad_ZeroTimeoutGetsDefault(t *testing.T) {
	dir := t.TempDir()
	content := `[explainer]
backend = "groq"
timeout_ms = 0
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Explainer.TimeoutMS != 5000 {
		t.Errorf("zero timeout_ms should fall back to 5000, got %d", cfg.Explainer.TimeoutMS)
	}
}
