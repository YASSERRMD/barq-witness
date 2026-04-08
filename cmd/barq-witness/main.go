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
)

const version = "vDEV"

func main() {
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
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
