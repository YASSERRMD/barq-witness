// Package config reads and holds the optional per-repository configuration
// from .witness/config.toml.
//
// All fields have safe defaults so the tool works without any config file.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// PluginEntry describes a single external plugin executable.
type PluginEntry struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

// Config is the top-level structure for .witness/config.toml.
type Config struct {
	Explainer ExplainerConfig `toml:"explainer"`
	Analyzer  AnalyzerConfig  `toml:"analyzer"`
	Privacy   PrivacyConfig   `toml:"privacy"`
	Sync      SyncConfig      `toml:"sync"`
	Plugins   []PluginEntry   `toml:"plugins"`
}

// ExplainerConfig controls the optional LLM explainer backend.
type ExplainerConfig struct {
	// Backend selects the LLM backend: null | claude | groq | local
	Backend string `toml:"backend"`
	// Model overrides the backend's default model.
	Model string `toml:"model"`
	// Endpoint is used by the local (Ollama) backend only.
	Endpoint string `toml:"endpoint"`
	// TimeoutMS is the per-request timeout in milliseconds.
	TimeoutMS int `toml:"timeout_ms"`
}

// AnalyzerConfig lets users extend or restrict the analyzer.
type AnalyzerConfig struct {
	// SecurityPathsExtra adds user-defined glob patterns to the security path list.
	SecurityPathsExtra []string `toml:"security_paths_extra"`
	// ExcludePaths lists file globs the analyzer should ignore entirely.
	ExcludePaths []string `toml:"exclude_paths"`
	// EnableIntentMatching enables the PROMPT_DIFF_MISMATCH signal (Phase 9).
	EnableIntentMatching bool `toml:"enable_intent_matching"`
	// IntentMatchThreshold is the score below which the signal fires (default 0.5).
	IntentMatchThreshold float64 `toml:"intent_match_threshold"`
}

// PrivacyConfig enforces privacy constraints globally.
type PrivacyConfig struct {
	// Mode forces privacy redaction everywhere, including exports and logs.
	Mode bool `toml:"mode"`
}

// SyncConfig controls the optional team aggregator sync (Phase 13).
type SyncConfig struct {
	// Enabled controls whether sync is active. Default false (opt-out).
	Enabled bool `toml:"enabled"`
	// ServerURL is the self-hosted aggregator endpoint.
	ServerURL string `toml:"server_url"`
	// AuthorUUID is the anonymized developer identifier (generated on first sync).
	AuthorUUID string `toml:"author_uuid"`
}

// Load reads the config from <witnessDir>/config.toml.
// If the file does not exist, Default() is returned.
// A parse error is returned as-is.
func Load(witnessDir string) (*Config, error) {
	cfg := Default()
	path := filepath.Join(witnessDir, "config.toml")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config.toml: %w", err)
	}

	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parse config.toml: %w", err)
	}

	// Apply defaults for zero-value fields.
	if cfg.Explainer.TimeoutMS <= 0 {
		cfg.Explainer.TimeoutMS = 5000
	}
	if cfg.Analyzer.IntentMatchThreshold == 0 {
		cfg.Analyzer.IntentMatchThreshold = 0.5
	}

	return cfg, nil
}

// Default returns a Config populated with safe defaults.
func Default() *Config {
	return &Config{
		Explainer: ExplainerConfig{
			Backend:   "null",
			TimeoutMS: 5000,
		},
		Analyzer: AnalyzerConfig{
			IntentMatchThreshold: 0.5,
		},
	}
}

// WitnessDir returns the standard .witness directory for the given repo root.
func WitnessDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".witness")
}
