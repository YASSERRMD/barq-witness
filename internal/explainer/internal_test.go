// Package-internal tests for functions that cannot be reached from the _test package.
package explainer

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
)

var internalDiscardLogger = log.New(io.Discard, "", 0)

// roundTripFunc is an http.RoundTripper adapter.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeRecorder captures what URL the client tried to call.
func fakeRecorder(srv *httptest.Server) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Redirect the request to the fake server.
		req2 := req.Clone(req.Context())
		req2.URL.Scheme = "http"
		req2.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req2)
	})
}

// TestClaudeExplainer_Explain_ViaCustomTransport tests ClaudeExplainer.Explain
// using an http.Client with a custom transport that redirects to a local test server.
func TestClaudeExplainer_Explain_ViaCustomTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":[{"type":"text","text":"Claude says: generated code skipped testing."}]}`))
	}))
	defer srv.Close()

	c := &ClaudeExplainer{
		apiKey:    "fake-key",
		model:     "claude-sonnet-4-6",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:   newLRUCache(10),
		logger:  internalDiscardLogger,
		privacy: false,
	}

	pID := int64(1)
	seg := analyzer.Segment{
		EditID:      1,
		FilePath:    "internal/auth/handler.go",
		LineStart:   10,
		LineEnd:     30,
		ReasonCodes: []string{"NO_EXEC"},
		PromptText:  "add authentication",
		PromptID:    &pID,
	}

	text, err := c.Explain(context.Background(), seg)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty explanation")
	}
}

// TestClaudeExplainer_Explain_CacheHit tests that the second call hits the cache.
func TestClaudeExplainer_Explain_CacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":[{"type":"text","text":"First response."}]}`))
	}))
	defer srv.Close()

	c := &ClaudeExplainer{
		apiKey:    "fake-key",
		model:     "test-model",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:   newLRUCache(10),
		logger:  internalDiscardLogger,
		privacy: false,
	}

	pID := int64(2)
	seg := analyzer.Segment{EditID: 2, FilePath: "main.go", PromptID: &pID}

	t1, _ := c.Explain(context.Background(), seg)
	t2, _ := c.Explain(context.Background(), seg)

	if t1 != t2 {
		t.Error("cache should return identical text on second call")
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}

// TestClaudeExplainer_Explain_Privacy tests privacy mode.
func TestClaudeExplainer_Explain_Privacy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":[{"type":"text","text":"Private response."}]}`))
	}))
	defer srv.Close()

	c := &ClaudeExplainer{
		apiKey:    "fake-key",
		model:     "test-model",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:   newLRUCache(10),
		logger:  internalDiscardLogger,
		privacy: true,
	}

	pID := int64(3)
	seg := analyzer.Segment{
		EditID:     3,
		FilePath:   "main.go",
		PromptText: "secret prompt",
		PromptID:   &pID,
	}

	_, err := c.Explain(context.Background(), seg)
	if err != nil {
		t.Fatalf("Explain with privacy: %v", err)
	}
}

// TestClaudeExplainer_IntentMatch tests IntentMatch with a fake server.
func TestClaudeExplainer_IntentMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":[{"type":"text","text":"{\"score\":0.9,\"reasoning\":\"Matches well.\"}"}]}`))
	}))
	defer srv.Close()

	c := &ClaudeExplainer{
		apiKey:    "fake-key",
		model:     "test-model",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:   newLRUCache(10),
		logger:  internalDiscardLogger,
		privacy: false,
	}

	result, err := c.IntentMatch(context.Background(), "add sort function", "diff text")
	if err != nil {
		t.Fatalf("IntentMatch: %v", err)
	}
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("score %v out of [0,1]", result.Score)
	}
}

// TestClaudeExplainer_callMessages_APIError handles server errors.
func TestClaudeExplainer_callMessages_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &ClaudeExplainer{
		apiKey:    "fake-key",
		model:     "test-model",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:  newLRUCache(10),
		logger: internalDiscardLogger,
	}

	_, err := c.callMessages(context.Background(), "test prompt", 100)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// TestExtractAnthropicText_NoTextBlock returns error when no text block present.
func TestExtractAnthropicText_NoTextBlock(t *testing.T) {
	raw := []byte(`{"content":[{"type":"image","data":"abc"}]}`)
	_, err := extractAnthropicText(raw)
	if err == nil {
		t.Error("expected error when no text block in response")
	}
}

// TestExtractAnthropicText_InvalidJSON returns error for bad JSON.
func TestExtractAnthropicText_InvalidJSON(t *testing.T) {
	_, err := extractAnthropicText([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestExtractAnthropicText_ValidResponse returns trimmed text.
func TestExtractAnthropicText_ValidResponse(t *testing.T) {
	raw := []byte(`{"content":[{"type":"text","text":"  hello world  "}]}`)
	text, err := extractAnthropicText(raw)
	if err != nil {
		t.Fatalf("extractAnthropicText: %v", err)
	}
	if text != "hello world" {
		t.Errorf("text = %q, want 'hello world'", text)
	}
}

// TestGroqExplainer_Explain_ViaCustomTransport tests GroqExplainer.Explain
// using a custom HTTP transport.
func TestGroqExplainer_Explain_ViaCustomTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"Groq explanation."}}]}`))
	}))
	defer srv.Close()

	g := &GroqExplainer{
		apiKey:    "fake-groq-key",
		model:     "llama-test",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:   newLRUCache(10),
		logger:  internalDiscardLogger,
		privacy: false,
	}

	pID := int64(10)
	seg := analyzer.Segment{
		EditID:      10,
		FilePath:    "groq.go",
		LineStart:   1,
		LineEnd:     20,
		ReasonCodes: []string{"NO_EXEC"},
		PromptID:    &pID,
	}

	text, err := g.Explain(context.Background(), seg)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty explanation from GroqExplainer")
	}
}

// TestGroqExplainer_Explain_CacheHit verifies caching.
func TestGroqExplainer_Explain_CacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"Groq cached."}}]}`))
	}))
	defer srv.Close()

	g := &GroqExplainer{
		apiKey:    "fake-groq-key",
		model:     "test-model",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:   newLRUCache(10),
		logger:  internalDiscardLogger,
		privacy: false,
	}

	pID := int64(11)
	seg := analyzer.Segment{EditID: 11, FilePath: "main.go", PromptID: &pID}

	t1, _ := g.Explain(context.Background(), seg)
	t2, _ := g.Explain(context.Background(), seg)

	if t1 != t2 {
		t.Error("cache should return identical text")
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}

// TestGroqExplainer_IntentMatch tests IntentMatch HTTP path.
func TestGroqExplainer_IntentMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"score\":0.8,\"reasoning\":\"Groq matches.\"}"}}]}`))
	}))
	defer srv.Close()

	g := &GroqExplainer{
		apiKey:    "fake-groq-key",
		model:     "test-model",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:   newLRUCache(10),
		logger:  internalDiscardLogger,
		privacy: true,
	}

	result, err := g.IntentMatch(context.Background(), "sort function", "diff")
	if err != nil {
		t.Fatalf("IntentMatch: %v", err)
	}
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("score %v out of [0,1]", result.Score)
	}
}

// TestGroqExplainer_callChat_APIError handles server errors.
func TestGroqExplainer_callChat_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	g := &GroqExplainer{
		apiKey:    "fake-groq-key",
		model:     "test-model",
		timeoutMS: 5000,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: fakeRecorder(srv),
		},
		cache:   newLRUCache(10),
		logger:  internalDiscardLogger,
		privacy: false,
	}

	_, err := g.callChat(context.Background(), "test prompt", 100)
	if err == nil {
		t.Error("expected error for 502 response")
	}
}

// TestExtractOpenAIText_ValidResponse parses a valid OpenAI response.
func TestExtractOpenAIText_ValidResponse(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"content":"  openai says hello  "}}]}`)
	text, err := extractOpenAIText(raw)
	if err != nil {
		t.Fatalf("extractOpenAIText: %v", err)
	}
	if text != "openai says hello" {
		t.Errorf("text = %q, want 'openai says hello'", text)
	}
}

// TestExtractOpenAIText_EmptyChoices returns error when choices is empty.
func TestExtractOpenAIText_EmptyChoices(t *testing.T) {
	raw := []byte(`{"choices":[]}`)
	_, err := extractOpenAIText(raw)
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

// TestExtractOpenAIText_InvalidJSON returns error for bad JSON.
func TestExtractOpenAIText_InvalidJSON(t *testing.T) {
	_, err := extractOpenAIText([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestExtractOllamaText_ValidResponse parses a valid Ollama response.
func TestExtractOllamaText_ValidResponse(t *testing.T) {
	raw := []byte(`{"message":{"content":"  ollama answer  "}}`)
	text, err := extractOllamaText(raw)
	if err != nil {
		t.Fatalf("extractOllamaText: %v", err)
	}
	if text != "ollama answer" {
		t.Errorf("text = %q, want 'ollama answer'", text)
	}
}

// TestExtractOllamaText_InvalidJSON returns error for bad JSON.
func TestExtractOllamaText_InvalidJSON(t *testing.T) {
	_, err := extractOllamaText([]byte("bad json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestLRUCache_ZeroCapacityDefaults to 128.
func TestLRUCache_ZeroCapacityDefaults(t *testing.T) {
	c := newLRUCache(0)
	if c.cap != 128 {
		t.Errorf("cap = %d, want 128 for zero input", c.cap)
	}
}

// TestLRUCache_SetEvictsOldest verifies eviction when at capacity.
func TestLRUCache_SetEvictsOldest(t *testing.T) {
	c := newLRUCache(2)
	c.Set("a", "val-a")
	c.Set("b", "val-b")
	// At capacity; adding "c" should evict "a".
	c.Set("c", "val-c")

	if _, ok := c.Get("a"); ok {
		t.Error("key 'a' should have been evicted")
	}
	if v, ok := c.Get("b"); !ok || v != "val-b" {
		t.Errorf("key 'b' should still be present, got ok=%v v=%q", ok, v)
	}
	if v, ok := c.Get("c"); !ok || v != "val-c" {
		t.Errorf("key 'c' should be present, got ok=%v v=%q", ok, v)
	}
}

// TestLRUCache_UpdateExisting updates value for an existing key without eviction.
func TestLRUCache_UpdateExisting(t *testing.T) {
	c := newLRUCache(3)
	c.Set("a", "val-a")
	c.Set("a", "val-a-updated")

	v, ok := c.Get("a")
	if !ok {
		t.Fatal("key 'a' should exist after update")
	}
	if v != "val-a-updated" {
		t.Errorf("value = %q, want 'val-a-updated'", v)
	}
	// Order slice should still have only one entry for "a".
	if len(c.order) != 1 {
		t.Errorf("order slice has %d entries, want 1 after update", len(c.order))
	}
}

// TestLRUCache_GetMiss returns false for missing key.
func TestLRUCache_GetMiss(t *testing.T) {
	c := newLRUCache(5)
	_, ok := c.Get("missing")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

// TestBuildExplainPrompt_PrivacyRedactsPrompt verifies privacy mode via logPrompt path.
func TestBuildExplainPrompt_PrivacyRedactsPrompt(t *testing.T) {
	seg := analyzer.Segment{
		FilePath:    "main.go",
		LineStart:   1,
		LineEnd:     10,
		PromptText:  "super secret prompt content",
		AcceptedSec: 5,
		Executed:    true,
		Modified:    true,
		ReasonCodes: []string{"NO_EXEC"},
	}

	withPrivacy := buildExplainPrompt(seg, true)
	withoutPrivacy := buildExplainPrompt(seg, false)

	if contains(withPrivacy, "super secret prompt content") {
		t.Error("privacy mode should redact prompt text")
	}
	if !contains(withPrivacy, "[redacted") {
		t.Error("privacy mode should contain [redacted]")
	}
	if !contains(withoutPrivacy, "super secret prompt content") {
		t.Error("non-privacy mode should include the real prompt text")
	}
}

// TestBuildExplainPrompt_NegativeAcceptedSec produces "unknown" in output.
func TestBuildExplainPrompt_NegativeAcceptedSec(t *testing.T) {
	seg := analyzer.Segment{
		FilePath:    "main.go",
		AcceptedSec: -1,
		ReasonCodes: []string{"NO_EXEC"},
	}
	prompt := buildExplainPrompt(seg, false)
	if !contains(prompt, "unknown") {
		t.Error("negative AcceptedSec should produce 'unknown' in prompt")
	}
}

// TestBuildIntentPrompt_ContainsBothInputs verifies both prompt and diff appear.
func TestBuildIntentPrompt_ContainsBothInputs(t *testing.T) {
	p := buildIntentPrompt("my prompt text", "my diff text")
	if !contains(p, "my prompt text") {
		t.Error("intent prompt should contain the prompt text")
	}
	if !contains(p, "my diff text") {
		t.Error("intent prompt should contain the diff text")
	}
}

// TestParseIntentJSON_ScoreAboveOneClamped verifies score > 1 is clamped to 1.
func TestParseIntentJSON_ScoreAboveOneClamped(t *testing.T) {
	result, err := parseIntentJSON(`{"score": 2.5, "reasoning": "too high"}`)
	if err != nil {
		t.Fatalf("parseIntentJSON: %v", err)
	}
	if result.Score != 1.0 {
		t.Errorf("score = %v, want 1.0 (clamped from 2.5)", result.Score)
	}
}

// TestParseIntentJSON_ScoreBelowZeroClamped verifies score < 0 is clamped to 0.
func TestParseIntentJSON_ScoreBelowZeroClamped(t *testing.T) {
	result, err := parseIntentJSON(`{"score": -0.5, "reasoning": "too low"}`)
	if err != nil {
		t.Fatalf("parseIntentJSON: %v", err)
	}
	if result.Score != 0.0 {
		t.Errorf("score = %v, want 0.0 (clamped from -0.5)", result.Score)
	}
}

// TestParseIntentJSON_NoBraces returns default score 1.0.
func TestParseIntentJSON_NoBraces(t *testing.T) {
	result, err := parseIntentJSON("no braces at all")
	if err != nil {
		t.Fatalf("parseIntentJSON: %v", err)
	}
	if result.Score != 1.0 {
		t.Errorf("expected 1.0 default score, got %v", result.Score)
	}
}

// TestParseIntentJSON_MalformedBraces returns default on bad JSON.
func TestParseIntentJSON_MalformedBraces(t *testing.T) {
	result, err := parseIntentJSON(`{bad json}`)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	// Should return 1.0 default with an error message in reasoning.
	if result.Score != 1.0 {
		t.Errorf("score = %v, want 1.0 default", result.Score)
	}
}

// contains is a simple substring helper.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
