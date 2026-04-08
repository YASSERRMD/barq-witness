package main

// airgap.go implements `barq-witness check-airgap`, which verifies that the
// current configuration is suitable for fully air-gapped operation.
//
// It checks four things and prints PASS or FAIL for each:
//  1. sync is disabled (sync.enabled = false or sync.server_url = "")
//  2. explainer backend is air-gap compatible (null, local, or edge)
//  3. Ollama is reachable at the configured endpoint (if backend is local/edge)
//  4. the configured model is present in Ollama (if backend is local/edge)
//
// Exit code is 0 if all checks pass, 1 if any fail.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yasserrmd/barq-witness/internal/config"
)

func runCheckAirgap(_ []string) {
	witnessDir := resolveWitnessDir()
	cfg, err := config.Load(witnessDir)
	if err != nil {
		fatalf("load config: %v", err)
	}

	allPass := true

	// Check 1: sync must be disabled.
	syncOK := !cfg.Sync.Enabled || cfg.Sync.ServerURL == ""
	printCheck("sync disabled", syncOK)
	if !syncOK {
		allPass = false
	}

	// Check 2: explainer backend must be air-gap compatible.
	backend := strings.ToLower(cfg.Explainer.Backend)
	backendOK := backend == "" || backend == "null" || backend == "local" || backend == "edge"
	printCheck("explainer backend is air-gap safe (null/local/edge)", backendOK)
	if !backendOK {
		allPass = false
	}

	// Checks 3 and 4 only apply when backend is local or edge.
	if backend == "local" || backend == "edge" {
		endpoint := cfg.Explainer.Endpoint
		if endpoint == "" {
			endpoint = "http://localhost:11434"
		}
		endpoint = strings.TrimRight(endpoint, "/")

		// Check 3: Ollama is reachable.
		ollamaOK, tagsBody := pingOllama(endpoint)
		printCheck("Ollama reachable at "+endpoint, ollamaOK)
		if !ollamaOK {
			allPass = false
		}

		// Check 4: configured model is present.
		model := cfg.Explainer.Model
		if model == "" {
			if backend == "edge" {
				model = "qwen2.5-coder:1.5b"
			} else {
				model = "liquid/lfm2.5-1.2b"
			}
		}
		modelOK := ollamaOK && modelPresent(tagsBody, model)
		printCheck("model "+model+" present in Ollama", modelOK)
		if !modelOK {
			allPass = false
		}
	}

	if !allPass {
		os.Exit(1)
	}
}

// printCheck prints a single PASS/FAIL line.
func printCheck(label string, ok bool) {
	status := "PASS"
	if !ok {
		status = "FAIL"
	}
	fmt.Printf("[%s] %s\n", status, label)
}

// pingOllama sends a GET /api/tags with a 1-second timeout.
// Returns (true, body) if the endpoint responds with HTTP 200, (false, nil) otherwise.
func pingOllama(endpoint string) (bool, []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/api/tags", nil)
	if err != nil {
		return false, nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil
	}
	return true, body
}

// modelPresent checks whether the given model name appears in the /api/tags response body.
func modelPresent(tagsBody []byte, model string) bool {
	if len(tagsBody) == 0 {
		return false
	}
	var resp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(tagsBody, &resp); err != nil {
		return false
	}
	for _, m := range resp.Models {
		if m.Name == model {
			return true
		}
	}
	return false
}
