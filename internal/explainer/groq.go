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
	groqDefaultModel  = "llama-3.3-70b-versatile"
	groqChatURL       = "https://api.groq.com/openai/v1/chat/completions"
	groqCacheCapacity = 256
)

// GroqExplainer calls Groq's OpenAI-compatible chat completions endpoint.
type GroqExplainer struct {
	apiKey    string
	model     string
	timeoutMS int
	client    *http.Client
	cache     *lruCache
	logger    *log.Logger
	privacy   bool
}

// NewGroq returns a GroqExplainer.  API key is read from GROQ_API_KEY.
func NewGroq(model string, timeoutMS int, logger *log.Logger, privacy bool) *GroqExplainer {
	if model == "" {
		model = groqDefaultModel
	}
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	return &GroqExplainer{
		apiKey:    os.Getenv("GROQ_API_KEY"),
		model:     model,
		timeoutMS: timeoutMS,
		client:    &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
		cache:     newLRUCache(groqCacheCapacity),
		logger:    logger,
		privacy:   privacy,
	}
}

func (g *GroqExplainer) Name() string    { return "groq" }
func (g *GroqExplainer) Available() bool { return g.apiKey != "" }
func (g *GroqExplainer) Close() error    { return nil }

// Explain returns a two-sentence explanation for the segment.
func (g *GroqExplainer) Explain(ctx context.Context, seg analyzer.Segment) (string, error) {
	cacheKey := fmt.Sprintf("%d:%s", seg.EditID, g.model)
	if cached, ok := g.cache.Get(cacheKey); ok {
		return cached, nil
	}

	prompt := buildExplainPrompt(seg, g.privacy)
	logged := prompt
	if g.privacy {
		logged = "[redacted -- privacy mode]"
	}
	g.logger.Printf("explainer=groq op=explain edit_id=%d prompt=%q", seg.EditID, logged)

	text, err := g.callChat(ctx, prompt, 200)
	if err != nil {
		return "", err
	}
	g.cache.Set(cacheKey, text)
	return text, nil
}

// IntentMatch scores how closely a diff matches the original prompt.
func (g *GroqExplainer) IntentMatch(ctx context.Context, prompt, diff string) (IntentResult, error) {
	p := buildIntentPrompt(prompt, diff)
	logged := prompt
	if g.privacy {
		logged = "[redacted]"
	}
	g.logger.Printf("explainer=groq op=intent_match prompt=%q", logged)

	raw, err := g.callChat(ctx, p, 100)
	if err != nil {
		return IntentResult{Score: 1.0, Reasoning: "intent match failed: " + err.Error()}, err
	}
	return parseIntentJSON(raw)
}

// callChat sends one turn to Groq's chat completions endpoint.
func (g *GroqExplainer) callChat(ctx context.Context, userContent string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      g.model,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": userContent},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqChatURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("content-type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq API: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq API status %d: %s", resp.StatusCode, raw)
	}

	return extractOpenAIText(raw)
}

// extractOpenAIText parses an OpenAI-compatible chat completions response.
func extractOpenAIText(raw []byte) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("parse openai response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in openai response")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// parseIntentJSON parses the JSON output from an IntentMatch prompt.
// It is tolerant of extra prose surrounding the JSON object.
func parseIntentJSON(raw string) (IntentResult, error) {
	// Find the first '{' and last '}'.
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return IntentResult{Score: 1.0, Reasoning: "could not parse intent JSON"}, nil
	}
	jsonStr := raw[start : end+1]

	var result struct {
		Score     float64 `json:"score"`
		Reasoning string  `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return IntentResult{Score: 1.0, Reasoning: "malformed intent JSON: " + err.Error()}, nil
	}
	// Clamp score to [0, 1].
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 1 {
		result.Score = 1
	}
	return IntentResult{Score: result.Score, Reasoning: result.Reasoning}, nil
}
