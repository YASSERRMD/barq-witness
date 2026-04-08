// barq-witness is a local-first provenance recorder for Claude Code sessions.
// It captures what Claude Code generates, what gets executed, and what the
// human modifies, then produces a deterministic risk-weighted attention map
// for code reviewers.
//
// Usage: barq-witness <command> [flags]
package main

import (
	"fmt"
	"os"
	"strings"
)

const version = "v0.1.0"

func main() {
	// Hidden flag used by `daemon start` to re-exec this binary as the server
	// process. Must be checked before normal dispatch.
	if len(os.Args) >= 2 && os.Args[1] == "--daemon-foreground" {
		witnessDir := resolveWitnessDir()
		for _, arg := range os.Args[2:] {
			if strings.HasPrefix(arg, "--witness-dir=") {
				witnessDir = strings.TrimPrefix(arg, "--witness-dir=")
			}
		}
		runDaemonForeground(witnessDir)
		return
	}

	if len(os.Args) < 2 {
		fmt.Printf("barq-witness %s\n", version)
		os.Exit(0)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("barq-witness %s\n", version)
	case "record":
		runRecord(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "report":
		runReport(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
	case "daemon":
		runDaemon(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
