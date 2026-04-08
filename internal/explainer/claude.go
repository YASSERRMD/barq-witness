package explainer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
)

const (
	claudeDefaultModel   = "claude-sonnet-4-6"
	claudeAPIBase        = "https://api.anthropic.com/v1/messages"
	claudeAPIVersion     = "2023-06-01"
	claudeCacheCapacity  = 256
)

// ClaudeExplainer calls the Anthropic Messages API via plain net/http.
// No SDK dependency.
type ClaudeExplainer struct {
	apiKey    string
	model     string
	timeoutMS int
	client    *http.Client
	cache     *lruCache
	logger    *log.Logger
	privacy   bool
}

// NewClaude returns a ClaudeExplainer.  API key is read from ANTHROPIC_API_KEY.
func NewClaude(model string, timeoutMS int, logger *log.Logger, privacy bool) *ClaudeExplainer {
	if model == "" {
		model = claudeDefaultModel
	}
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	return &ClaudeExplainer{
		apiKey:    os.Getenv("ANTHROPIC_API_KEY"),
		model:     model,
		timeoutMS: timeoutMS,
		client:    &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
		cache:     newLRUCache(claudeCacheCapacity),
		logger:    logger,
		privacy:   privacy,
	}
}

func (c *ClaudeExplainer) Name() string    { return "claude" }
func (c *ClaudeExplainer) Available() bool { return c.apiKey != "" }
func (c *ClaudeExplainer) Close() error    { return nil }

// Explain returns a two-sentence explanation for the segment.
// Results are cached by (editID, model).
func (c *ClaudeExplainer) Explain(ctx context.Context, seg analyzer.Segment) (string, error) {
	cacheKey := fmt.Sprintf("%d:%s", seg.EditID, c.model)
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached, nil
	}

	prompt := buildExplainPrompt(seg, c.privacy)
	c.logPrompt("explain", seg.EditID, prompt)

	text, err := c.callMessages(ctx, prompt, 200)
	if err != nil {
		return "", err
	}
	c.cache.Set(cacheKey, text)
	return text, nil
}

// IntentMatch scores how closely a diff matches the original prompt.
func (c *ClaudeExplainer) IntentMatch(ctx context.Context, prompt, diff string) (IntentResult, error) {
	p := buildIntentPrompt(prompt, diff)
	logPrompt := prompt
	if c.privacy {
		logPrompt = "[redacted]"
	}
	c.logger.Printf("explainer=claude op=intent_match prompt=%q", logPrompt)

	raw, err := c.callMessages(ctx, p, 100)
	if err != nil {
		return IntentResult{Score: 1.0, Reasoning: "intent match failed: " + err.Error()}, err
	}
	return parseIntentJSON(raw)
}

// callMessages sends one user turn to the Anthropic Messages API.
func (c *ClaudeExplainer) callMessages(ctx context.Context, userContent string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      c.model,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": userContent},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIBase, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", claudeAPIVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude API: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude API status %d: %s", resp.StatusCode, raw)
	}

	return extractAnthropicText(raw)
}

// extractAnthropicText parses the Anthropic Messages response.
func extractAnthropicText(raw []byte) (string, error) {
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("parse claude response: %w", err)
	}
	for _, block := range resp.Content {
		if block.Type == "text" {
			return strings.TrimSpace(block.Text), nil
		}
	}
	return "", fmt.Errorf("no text block in claude response")
}

func (c *ClaudeExplainer) logPrompt(op string, editID int64, prompt string) {
	logged := prompt
	if c.privacy {
		logged = "[redacted -- privacy mode]"
	}
	c.logger.Printf("explainer=claude op=%s edit_id=%d prompt_len=%d prompt=%q",
		op, editID, len(prompt), logged)
}
