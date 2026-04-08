package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/installer"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// runInit implements `barq-witness init [--force]`.
func runInit(args []string) {
	force := false
	for _, a := range args {
		if a == "--force" {
			force = true
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fatalf("cannot determine working directory: %v", err)
	}

	// 1. Verify this is a git repository.
	if !isGitRepo(cwd) {
		fatalf("not a git repository (no .git found in %s)\nRun `git init` first.", cwd)
	}

	// 2. Create .witness/ directory.
	witnessDir := filepath.Join(cwd, ".witness")
	if err := os.MkdirAll(witnessDir, 0o755); err != nil {
		fatalf("create .witness/: %v", err)
	}

	// 3. Create / verify the trace database (schema auto-applied).
	dbPath := filepath.Join(witnessDir, "trace.db")
	s, err := store.Open(dbPath)
	if err != nil {
		fatalf("open trace database: %v", err)
	}
	s.Close()

	// 4. Ensure .witness/ is in .gitignore.
	gitignorePath := filepath.Join(cwd, ".gitignore")
	added, err := ensureGitignoreEntry(gitignorePath, ".witness/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// 5. Merge hook entries into .claude/settings.json.
	settingsPath := filepath.Join(cwd, ".claude", "settings.json")
	result, err := installer.Install(settingsPath, force)
	if err != nil {
		fatalf("install hooks: %v", err)
	}

	// 6. Print summary.
	fmt.Println("barq-witness init complete")
	fmt.Println()
	fmt.Printf("  Trace database : %s\n", dbPath)
	fmt.Printf("  Settings file  : %s\n", settingsPath)

	if added {
		fmt.Printf("  .gitignore     : added .witness/ entry\n")
	} else {
		fmt.Printf("  .gitignore     : .witness/ already present\n")
	}

	fmt.Println()
	if len(result.Added) > 0 {
		fmt.Printf("  Hooks installed for: %s\n", strings.Join(result.Added, ", "))
	}
	if len(result.Skipped) > 0 {
		fmt.Printf("  Hooks already present (skipped): %s\n", strings.Join(result.Skipped, ", "))
		if !force {
			fmt.Println("  Run with --force to replace existing barq-witness hooks.")
		}
	}

	fmt.Println()
	fmt.Println("Use Claude Code normally; barq-witness will capture the trace automatically.")
	fmt.Println("Run `barq-witness report` after a commit to see the attention map.")
}

// isGitRepo returns true if dir or any ancestor contains a .git entry.
func isGitRepo(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// ensureGitignoreEntry appends entry to path if it is not already present.
// Returns true if the entry was added, false if it was already there.
func ensureGitignoreEntry(path, entry string) (bool, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == entry {
			return false, scanner.Err()
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}

	// Append the entry.
	if _, err := fmt.Fprintf(f, "\n%s\n", entry); err != nil {
		return false, err
	}
	return true, nil
}

// fatalf prints an error to stderr and exits with code 1.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
