// license-header-check is a barq-witness plugin that checks that new Go files
// introduced in a diff carry a license header.
//
// Detection logic:
//   - Look for lines starting with "+++" that reference a .go file (new file in diff).
//   - If found, check whether the diff text contains "// Copyright" or
//     "// SPDX-License-Identifier".
//   - If a new Go file is detected but no header marker is present, emit
//     signal plugin:MISSING_LICENSE_HEADER at tier 3.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

// CheckMissingLicenseHeader returns true when the diff text appears to
// introduce a new .go file but does not contain a license header marker.
// This function is package-level so tests can call it directly.
func CheckMissingLicenseHeader(diff string) bool {
	hasNewGoFile := false
	for _, line := range strings.Split(diff, "\n") {
		// Unified diff header for a new file looks like:
		//   +++ b/some/path/file.go
		if strings.HasPrefix(line, "+++") && strings.HasSuffix(strings.TrimSpace(line), ".go") {
			hasNewGoFile = true
			break
		}
	}
	if !hasNewGoFile {
		return false
	}
	return !strings.Contains(diff, "// Copyright") && !strings.Contains(diff, "// SPDX-License-Identifier")
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
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
		if CheckMissingLicenseHeader(req.Segment.PromptText) {
			signals = append(signals, signal{
				Code:    "plugin:MISSING_LICENSE_HEADER",
				Tier:    3,
				Message: "New Go file in diff is missing a license header (// Copyright or // SPDX-License-Identifier)",
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
