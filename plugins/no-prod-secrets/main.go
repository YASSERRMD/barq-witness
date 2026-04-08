// no-prod-secrets is a barq-witness plugin that detects production secret
// patterns in diffs. It reads one JSON line from stdin and writes a JSON
// result line to stdout.
//
// Detected patterns:
//   - AWS access key IDs: AKIA[0-9A-Z]{16}
//   - Generic secret assignments: (password|secret|api_key|token)=["'][^"']{8,}
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// request mirrors the host protocol message (only the fields we need).
type request struct {
	Segment struct {
		PromptText string `json:"prompt_text"`
	} `json:"segment"`
}

// signal is an individual risk signal.
type signal struct {
	Code    string `json:"code"`
	Tier    int    `json:"tier"`
	Message string `json:"message"`
}

// response is the plugin's stdout JSON line.
type response struct {
	Signals []signal `json:"signals"`
	Error   *string  `json:"error"`
}

var (
	reAWSKey = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	reSecret = regexp.MustCompile(`(?i)(password|secret|api_key|token)\s*=\s*["'][^"']{8,}`)
)

// ScanForSecrets checks text for production secret patterns and returns true
// if any are found. This is exported (by convention through a wrapper) so
// that tests can call it directly.
func ScanForSecrets(text string) bool {
	return reAWSKey.MatchString(text) || reSecret.MatchString(text)
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	// Increase scanner buffer for large diffs.
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			errStr := fmt.Sprintf("parse request: %v", err)
			writeResponse(response{Signals: []signal{}, Error: &errStr})
			continue
		}

		var signals []signal
		if ScanForSecrets(req.Segment.PromptText) {
			signals = append(signals, signal{
				Code:    "plugin:NO_PROD_SECRETS",
				Tier:    1,
				Message: "Found secret pattern (AWS key or credential assignment) in diff",
			})
		}

		if signals == nil {
			signals = []signal{}
		}
		writeResponse(response{Signals: signals, Error: nil})
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scanner error: %v\n", err)
		os.Exit(1)
	}
}

func writeResponse(r response) {
	data, err := json.Marshal(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal response: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
}
