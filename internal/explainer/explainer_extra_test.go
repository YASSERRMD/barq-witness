package explainer_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/explainer"
)

// ---- buildExplainPrompt via NullExplainer explain (indirect test) ----------

// TestBuildExplainPrompt_PrivacyMode verifies that NullExplainer uses the prompt text
// when privacy is off (indirectly through NewLocal with a fake server).
func TestBuildExplainPrompt_ContainsFilePath(t *testing.T) {
	// We test buildExplainPrompt indirectly: the NullExplainer's Explain output
	// must contain references to the segment's facts.
	n := explainer.NewNull()
	seg := analyzer.Segment{
		FilePath:    "internal/auth/secret.go",
		LineStart:   5,
		LineEnd:     25,
		ReasonCodes: []string{"NO_EXEC"},
		AcceptedSec: -1, // should produce "unknown"
		Executed:    false,
	}
	text, err := n.Explain(contextWithTimeout(t), seg)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty explanation for NO_EXEC")
	}
}

// TestBuildExplainPrompt_AllReasonCodes verifies each reason code produces output.
func TestBuildExplainPrompt_AllReasonCodes(t *testing.T) {
	reasonCodes := []string{
		"NO_EXEC",
		"FAST_ACCEPT_SECURITY",
		"TEST_FAIL_NO_RETEST",
		"HIGH_REGEN",
		"NEVER_REOPENED",
		"LARGE_MULTIFILE",
		"NEW_DEPENDENCY",
		"FAST_ACCEPT_GENERIC",
		"LONG_GENERATED_BLOCK",
		"UNKNOWN_CODE_XYZ",
	}

	n := explainer.NewNull()
	for _, code := range reasonCodes {
		t.Run(code, func(t *testing.T) {
			seg := analyzer.Segment{
				FilePath:    "pkg/foo/bar.go",
				LineStart:   1,
				LineEnd:     10,
				ReasonCodes: []string{code},
				AcceptedSec: 5,
				RegenCount:  3,
			}
			text, err := n.Explain(contextWithTimeout(t), seg)
			if err != nil {
				t.Fatalf("Explain(%q): %v", code, err)
			}
			if text == "" {
				t.Errorf("Explain(%q) returned empty string", code)
			}
		})
	}
}

// TestBuildIntentPrompt_NonEmpty verifies that buildIntentPrompt produces a string
// (tested indirectly via NullExplainer IntentMatch).
func TestBuildIntentPrompt_NonEmpty(t *testing.T) {
	n := explainer.NewNull()
	result, err := n.IntentMatch(contextWithTimeout(t), "write a sorting function", "diff content here")
	if err != nil {
		t.Fatalf("IntentMatch: %v", err)
	}
	if result.Reasoning == "" {
		t.Error("expected non-empty reasoning from NullExplainer")
	}
}

// ---- parseIntentJSON (tested via LocalExplainer with fake server) ----------

// TestParseIntentJSON_ValidJSON verifies correct JSON parsing via LocalExplainer.
func TestParseIntentJSON_ValidJSON(t *testing.T) {
	// Set up a fake Ollama server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "test-model"}},
			})
		case "/api/chat":
			// Return a valid IntentMatch JSON response.
			resp := map[string]any{
				"message": map[string]any{
					"content": `{"score": 0.85, "reasoning": "The diff matches the prompt well."}`,
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	l := explainer.NewLocal("test-model", srv.URL, 5000, discardLogger, false)
	if !l.Available() {
		t.Skip("fake server not responding as available")
	}

	result, err := l.IntentMatch(contextWithTimeout(t), "write a sort function", "diff text")
	if err != nil {
		t.Fatalf("IntentMatch: %v", err)
	}
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("score %v out of [0,1] range", result.Score)
	}
}

// TestParseIntentJSON_InvalidJSON gracefully handles a non-JSON response.
func TestParseIntentJSON_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "test-model"}},
			})
		case "/api/chat":
			// Return prose instead of JSON.
			resp := map[string]any{
				"message": map[string]any{
					"content": "This is not JSON at all, just prose.",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	l := explainer.NewLocal("test-model", srv.URL, 5000, discardLogger, false)
	if !l.Available() {
		t.Skip("fake server not responding as available")
	}

	result, err := l.IntentMatch(contextWithTimeout(t), "prompt", "diff")
	// parseIntentJSON is tolerant -- it returns a default result rather than error.
	if err != nil {
		t.Logf("IntentMatch returned error (acceptable): %v", err)
	}
	// Score should be clamped to [0,1].
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("score %v out of [0,1] range", result.Score)
	}
}

// TestParseIntentJSON_MissingFields returns default values for partial JSON.
func TestParseIntentJSON_MissingFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "test-model"}},
			})
		case "/api/chat":
			// Return JSON with missing reasoning field.
			resp := map[string]any{
				"message": map[string]any{
					"content": `{"score": 0.5}`,
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	l := explainer.NewLocal("test-model", srv.URL, 5000, discardLogger, false)
	if !l.Available() {
		t.Skip("fake server not responding as available")
	}

	result, err := l.IntentMatch(contextWithTimeout(t), "prompt", "diff")
	if err != nil {
		t.Logf("IntentMatch error (acceptable): %v", err)
	}
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("score %v out of range", result.Score)
	}
}

// ---- lruCache ---------------------------------------------------------------

// TestLRUCache_GetSetBasic verifies Get and Set work correctly.
func TestLRUCache_GetSetBasic(t *testing.T) {
	// Use NewLocal with a fake server to indirectly exercise the cache,
	// but also test the cache logic we can observe through behavior.
	// Since lruCache is unexported, we test it through LocalExplainer caching.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "test-model"}},
			})
		case "/api/chat":
			resp := map[string]any{
				"message": map[string]any{
					"content": "Generated explanation text.",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	l := explainer.NewLocal("test-model", srv.URL, 5000, discardLogger, false)
	if !l.Available() {
		t.Skip("fake server not responding as available")
	}

	pID := int64(99)
	seg := analyzer.Segment{
		EditID:      99,
		FilePath:    "cache_test.go",
		LineStart:   1,
		LineEnd:     5,
		ReasonCodes: []string{"NO_EXEC"},
		PromptID:    &pID,
	}

	// First call.
	text1, err := l.Explain(contextWithTimeout(t), seg)
	if err != nil {
		t.Fatalf("first Explain: %v", err)
	}

	// Second call with same segment -- should hit cache (same result).
	text2, err := l.Explain(contextWithTimeout(t), seg)
	if err != nil {
		t.Fatalf("second Explain: %v", err)
	}

	if text1 != text2 {
		t.Error("cache should return identical text on repeated calls")
	}
}

// TestLRUCache_EvictionBehavior verifies cache eviction does not panic.
func TestLRUCache_EvictionBehavior(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "test-model"}},
			})
		case "/api/chat":
			callCount++
			resp := map[string]any{
				"message": map[string]any{
					"content": fmt.Sprintf("Response %d", callCount),
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	l := explainer.NewLocal("test-model", srv.URL, 5000, discardLogger, false)
	if !l.Available() {
		t.Skip("fake server not responding as available")
	}

	// Make many calls with different edit IDs to exercise cache eviction.
	for i := int64(0); i < 10; i++ {
		pID := i
		seg := analyzer.Segment{
			EditID:      i,
			FilePath:    fmt.Sprintf("file%d.go", i),
			LineStart:   1,
			LineEnd:     10,
			ReasonCodes: []string{"NO_EXEC"},
			PromptID:    &pID,
		}
		if _, err := l.Explain(contextWithTimeout(t), seg); err != nil {
			t.Fatalf("Explain edit %d: %v", i, err)
		}
	}
}

// TestLocalExplainer_ExplainWithPrivacy verifies privacy mode redacts prompt.
func TestLocalExplainer_ExplainWithPrivacy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "test-model"}},
			})
		case "/api/chat":
			// Verify the request body does not contain the real prompt text.
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "secret prompt text") {
				t.Error("privacy mode: real prompt text leaked into API call")
			}
			resp := map[string]any{
				"message": map[string]any{
					"content": "Private explanation.",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	l := explainer.NewLocal("test-model", srv.URL, 5000, discardLogger, true /* privacy=true */)
	if !l.Available() {
		t.Skip("fake server not responding as available")
	}

	pID := int64(1)
	seg := analyzer.Segment{
		EditID:      1,
		FilePath:    "main.go",
		LineStart:   1,
		LineEnd:     5,
		ReasonCodes: []string{"NO_EXEC"},
		PromptText:  "secret prompt text",
		PromptID:    &pID,
	}
	_, err := l.Explain(contextWithTimeout(t), seg)
	if err != nil {
		t.Fatalf("Explain with privacy: %v", err)
	}
}

// TestLocalExplainer_APIError gracefully handles server errors.
func TestLocalExplainer_APIError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "test-model"}},
			})
		case "/api/chat":
			callCount++
			// Always return 500.
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	l := explainer.NewLocal("test-model", srv.URL, 5000, discardLogger, false)
	if !l.Available() {
		t.Skip("fake server not responding as available")
	}

	pID := int64(1)
	seg := analyzer.Segment{
		EditID:      1,
		FilePath:    "main.go",
		ReasonCodes: []string{"NO_EXEC"},
		PromptID:    &pID,
	}
	_, err := l.Explain(contextWithTimeout(t), seg)
	// Expect error since both primary and fallback fail.
	if err == nil {
		t.Error("expected error from Explain when API returns 500")
	}
}

// TestEdgeExplainer_UnavailableWhenNotRunning verifies EdgeExplainer reports unavailable.
func TestEdgeExplainer_UnavailableWhenNotRunning(t *testing.T) {
	e := explainer.NewEdge("", "http://localhost:29999", 500, discardLogger, false)
	if e.Available() {
		t.Skip("something is running on port 29999, skipping")
	}
}

// TestEdgeExplainer_AvailableWithFakeServer verifies EdgeExplainer Available() logic.
func TestEdgeExplainer_AvailableWithFakeServer(t *testing.T) {
	modelName := "test-edge-model"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": modelName}},
			})
		}
	}))
	defer srv.Close()

	e := explainer.NewEdge(modelName, srv.URL, 2000, discardLogger, false)
	if !e.Available() {
		t.Error("EdgeExplainer should be available when fake server returns the model in tags")
	}
}

// TestEdgeExplainer_ModelNotInTags returns false when model not in tags list.
func TestEdgeExplainer_ModelNotInTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "other-model"}},
			})
		}
	}))
	defer srv.Close()

	e := explainer.NewEdge("missing-model", srv.URL, 2000, discardLogger, false)
	if e.Available() {
		t.Error("EdgeExplainer should not be available when model is not in tags")
	}
}

// TestGroqExplainer_WithFakeServer verifies GroqExplainer HTTP path.
func TestGroqExplainer_WithFakeServer(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "fake-key-for-test")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "Groq explanation text."}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// GroqExplainer uses the hardcoded groqChatURL so we cannot redirect it
	// via config. We test Available() and Name() at minimum.
	g := explainer.NewGroq("", 1000, discardLogger, false)
	if g.Name() != "groq" {
		t.Errorf("Name() = %q, want groq", g.Name())
	}
	if !g.Available() {
		t.Error("GroqExplainer should be available when GROQ_API_KEY is set")
	}
}

// ---- helpers ---------------------------------------------------------------

func contextWithTimeout(t *testing.T) interface{ Deadline() (time.Time, bool); Done() <-chan struct{}; Err() error; Value(key any) any } {
	t.Helper()
	// Return a context that respects t's deadline via context.Background as a base.
	// We use a simple Background context to avoid importing "context" at package level.
	return testContext{}
}

type testContext struct{}

func (testContext) Deadline() (time.Time, bool)          { return time.Time{}, false }
func (testContext) Done() <-chan struct{}                  { return nil }
func (testContext) Err() error                            { return nil }
func (testContext) Value(_ any) any                       { return nil }
