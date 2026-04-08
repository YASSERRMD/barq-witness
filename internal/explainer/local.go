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
	localDefaultEndpoint = "http://localhost:11434"
	localDefaultModel    = "liquid/lfm2.5-1.2b"
	localFallbackModel   = "qwen2.5-coder:1.5b"
	localCacheCapacity   = 256
)

// LocalExplainer calls an Ollama-compatible local inference endpoint.
type LocalExplainer struct {
	endpoint  string
	model     string
	timeoutMS int
	client    *http.Client
	cache     *lruCache
	logger    *log.Logger
	privacy   bool
}

// NewLocal returns a LocalExplainer.
// If endpoint is empty, http://localhost:11434 is used.
// If model is empty, localDefaultModel is used.
func NewLocal(model, endpoint string, timeoutMS int, logger *log.Logger, privacy bool) *LocalExplainer {
	if endpoint == "" {
		endpoint = localDefaultEndpoint
	}
	if model == "" {
		model = localDefaultModel
	}
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	return &LocalExplainer{
		endpoint:  strings.TrimRight(endpoint, "/"),
		model:     model,
		timeoutMS: timeoutMS,
		client:    &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
		cache:     newLRUCache(localCacheCapacity),
		logger:    logger,
		privacy:   privacy,
	}
}

func (l *LocalExplainer) Name() string { return "local" }

// Available pings the Ollama /api/tags endpoint with a 1-second timeout.
func (l *LocalExplainer) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.endpoint+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (l *LocalExplainer) Close() error { return nil }

// Explain returns a two-sentence explanation for the segment.
func (l *LocalExplainer) Explain(ctx context.Context, seg analyzer.Segment) (string, error) {
	cacheKey := fmt.Sprintf("%d:%s", seg.EditID, l.model)
	if cached, ok := l.cache.Get(cacheKey); ok {
		return cached, nil
	}

	prompt := buildExplainPrompt(seg, l.privacy)
	logged := prompt
	if l.privacy {
		logged = "[redacted -- privacy mode]"
	}
	l.logger.Printf("explainer=local op=explain edit_id=%d model=%s prompt=%q",
		seg.EditID, l.model, logged)

	text, err := l.callChat(ctx, prompt, 200)
	if err != nil {
		// Attempt fallback model.
		l.logger.Printf("explainer=local fallback to %s after error: %v", localFallbackModel, err)
		l.model = localFallbackModel
		text, err = l.callChat(ctx, prompt, 200)
		if err != nil {
			return "", err
		}
	}
	l.cache.Set(cacheKey, text)
	return text, nil
}

// IntentMatch scores how closely a diff matches the original prompt.
func (l *LocalExplainer) IntentMatch(ctx context.Context, prompt, diff string) (IntentResult, error) {
	p := buildIntentPrompt(prompt, diff)
	logged := prompt
	if l.privacy {
		logged = "[redacted]"
	}
	l.logger.Printf("explainer=local op=intent_match prompt=%q", logged)

	raw, err := l.callChat(ctx, p, 150)
	if err != nil {
		return IntentResult{Score: 1.0, Reasoning: "local intent match failed: " + err.Error()}, err
	}
	return parseIntentJSON(raw)
}

// callChat calls the Ollama /api/chat endpoint with stream=false.
func (l *LocalExplainer) callChat(ctx context.Context, userContent string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":  l.model,
		"stream": false,
		"options": map[string]any{
			"num_predict": maxTokens,
		},
		"messages": []map[string]string{
			{"role": "user", "content": userContent},
		},
	})

	url := l.endpoint + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("local API: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("local API status %d: %s", resp.StatusCode, raw)
	}

	return extractOllamaText(raw)
}

// extractOllamaText parses an Ollama /api/chat response.
func extractOllamaText(raw []byte) (string, error) {
	var resp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("parse ollama response: %w", err)
	}
	return strings.TrimSpace(resp.Message.Content), nil
}
