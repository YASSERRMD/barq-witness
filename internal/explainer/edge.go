package explainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
)

const (
	edgeDefaultModel    = "qwen2.5-coder:1.5b"
	edgeDefaultTimeout  = 3000
	edgeMaxTokens       = 100
	edgeCacheCapacity   = 256
)

// EdgeExplainer calls an Ollama-compatible local inference endpoint,
// optimized for edge / air-gapped environments with smaller models.
// It uses qwen2.5-coder:1.5b by default, a shorter timeout, and a
// lower max_tokens cap to keep responses concise on constrained hardware.
type EdgeExplainer struct {
	endpoint  string
	model     string
	timeoutMS int
	client    *http.Client
	cache     *lruCache
	logger    *log.Logger
	privacy   bool
}

// NewEdge returns an EdgeExplainer.
// If endpoint is empty, http://localhost:11434 is used.
// If model is empty, edgeDefaultModel is used.
// If timeoutMS is <= 0, edgeDefaultTimeout is used.
func NewEdge(model, endpoint string, timeoutMS int, logger *log.Logger, privacy bool) *EdgeExplainer {
	if endpoint == "" {
		endpoint = localDefaultEndpoint
	}
	if model == "" {
		model = edgeDefaultModel
	}
	if timeoutMS <= 0 {
		timeoutMS = edgeDefaultTimeout
	}
	return &EdgeExplainer{
		endpoint:  strings.TrimRight(endpoint, "/"),
		model:     model,
		timeoutMS: timeoutMS,
		client:    &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
		cache:     newLRUCache(edgeCacheCapacity),
		logger:    logger,
		privacy:   privacy,
	}
}

func (e *EdgeExplainer) Name() string { return "edge" }

// Available checks two things:
// (a) pings /api/tags within 1 second to verify Ollama is running, and
// (b) parses the tags JSON to confirm the configured model is present.
// Returns true only if both checks pass.
func (e *EdgeExplainer) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.endpoint+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Parse the tags response to check if the configured model is present.
	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &tagsResp); err != nil {
		return false
	}
	for _, m := range tagsResp.Models {
		if m.Name == e.model {
			return true
		}
	}
	return false
}

func (e *EdgeExplainer) Close() error { return nil }

// Explain returns a two-sentence explanation for the segment.
func (e *EdgeExplainer) Explain(ctx context.Context, seg analyzer.Segment) (string, error) {
	cacheKey := fmt.Sprintf("%d:%s", seg.EditID, e.model)
	if cached, ok := e.cache.Get(cacheKey); ok {
		return cached, nil
	}

	prompt := buildExplainPrompt(seg, e.privacy)
	logged := prompt
	if e.privacy {
		logged = "[redacted -- privacy mode]"
	}
	e.logger.Printf("explainer=edge op=explain edit_id=%d model=%s prompt=%q",
		seg.EditID, e.model, logged)

	text, err := e.callChat(ctx, prompt, edgeMaxTokens)
	if err != nil {
		return "", err
	}
	e.cache.Set(cacheKey, text)
	return text, nil
}

// IntentMatch scores how closely a diff matches the original prompt.
func (e *EdgeExplainer) IntentMatch(ctx context.Context, prompt, diff string) (IntentResult, error) {
	p := buildIntentPrompt(prompt, diff)
	logged := prompt
	if e.privacy {
		logged = "[redacted]"
	}
	e.logger.Printf("explainer=edge op=intent_match prompt=%q", logged)

	raw, err := e.callChat(ctx, p, edgeMaxTokens)
	if err != nil {
		return IntentResult{Score: 1.0, Reasoning: "edge intent match failed: " + err.Error()}, err
	}
	return parseIntentJSON(raw)
}

// callChat calls the Ollama /api/chat endpoint with stream=false.
func (e *EdgeExplainer) callChat(ctx context.Context, userContent string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":  e.model,
		"stream": false,
		"options": map[string]any{
			"num_predict": maxTokens,
		},
		"messages": []map[string]string{
			{"role": "user", "content": userContent},
		},
	})

	url := e.endpoint + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("edge API: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("edge API status %d: %s", resp.StatusCode, raw)
	}

	return extractOllamaText(raw)
}
