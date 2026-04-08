package installer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yasserrmd/barq-witness/internal/installer"
)

// freshSettingsPath returns a path inside a temp dir (file does not exist yet).
func freshSettingsPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".claude", "settings.json")
}

// writeSettings creates the settings file at path with the given JSON content.
func writeSettings(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// allBarqEventsInstalled asserts that all four barq-witness hook events appear
// in the settings file.
func allBarqEventsInstalled(t *testing.T, path string) {
	t.Helper()
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "SessionEnd"} {
		groups, err := installer.ReadHookGroups(path, event)
		if err != nil {
			t.Fatalf("ReadHookGroups(%s): %v", event, err)
		}
		if !installer.HasBarqHooks(groups) {
			t.Errorf("expected barq-witness hooks for event %s", event)
		}
	}
}

// TestInstall_FreshRepo verifies installation when no settings.json exists.
func TestInstall_FreshRepo(t *testing.T) {
	path := freshSettingsPath(t)
	result, err := installer.Install(path, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(result.Added) == 0 {
		t.Error("expected at least one event to be added")
	}
	if len(result.Skipped) != 0 {
		t.Errorf("expected no skipped events, got %v", result.Skipped)
	}
	allBarqEventsInstalled(t, path)
}

// TestInstall_ExistingHooks verifies that pre-existing user hooks are preserved.
func TestInstall_ExistingHooks(t *testing.T) {
	path := freshSettingsPath(t)
	writeSettings(t, path, `{
  "hooks": {
    "SessionStart": [
      { "hooks": [ { "type": "command", "command": "my-other-tool session-start" } ] }
    ]
  },
  "custom_key": "preserved"
}`)

	_, err := installer.Install(path, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// barq-witness hooks must be present.
	allBarqEventsInstalled(t, path)

	// Pre-existing user hook must still be there.
	raw, _ := os.ReadFile(path)
	if !jsonContainsCommand(raw, "my-other-tool session-start") {
		t.Error("pre-existing hook was clobbered")
	}

	// Unrelated top-level key must be preserved.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if _, ok := doc["custom_key"]; !ok {
		t.Error("custom_key was removed from settings.json")
	}
}

// TestInstall_Idempotent verifies that running Install twice does not duplicate hooks.
func TestInstall_Idempotent(t *testing.T) {
	path := freshSettingsPath(t)

	if _, err := installer.Install(path, false); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	result, err := installer.Install(path, false)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}

	if len(result.Added) != 0 {
		t.Errorf("second run should add nothing, added: %v", result.Added)
	}
	if len(result.Skipped) == 0 {
		t.Error("second run should report all events as skipped")
	}

	// File must still be valid and contain exactly one barq-witness entry per event.
	allBarqEventsInstalled(t, path)
}

// TestInstall_ForceOverwrite verifies --force replaces existing barq-witness hooks.
func TestInstall_ForceOverwrite(t *testing.T) {
	path := freshSettingsPath(t)

	// First install.
	if _, err := installer.Install(path, false); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	// Force re-install.
	result, err := installer.Install(path, true)
	if err != nil {
		t.Fatalf("force Install: %v", err)
	}
	if len(result.Added) == 0 {
		t.Error("force Install should report added events")
	}

	allBarqEventsInstalled(t, path)
}

// TestInstall_EmptyFile handles an empty existing settings.json.
func TestInstall_EmptyFile(t *testing.T) {
	path := freshSettingsPath(t)
	writeSettings(t, path, "")

	_, err := installer.Install(path, false)
	if err != nil {
		t.Fatalf("Install on empty file: %v", err)
	}
	allBarqEventsInstalled(t, path)
}

// ---- helpers ----------------------------------------------------------------

func jsonContainsCommand(raw []byte, cmd string) bool {
	return containsString(string(raw), cmd)
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			len(haystack) > 0 && findSubstring(haystack, needle))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
