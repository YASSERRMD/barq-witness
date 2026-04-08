package main

// sync.go implements `barq-witness sync`, which exports a CGPF document and
// POSTs it to a self-hosted barq-witness-server instance.
//
// Configuration is read from .witness/config.toml:
//
//   [sync]
//   enabled     = true
//   server_url  = "http://localhost:8080"
//   author_uuid = "550e8400-e29b-41d4-a716-446655440000"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/yasserrmd/barq-witness/internal/cgpf"
	"github.com/yasserrmd/barq-witness/internal/config"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// runSync implements `barq-witness sync [flags]`.
func runSync(args []string) {
	privacy := false
	for _, a := range args {
		if a == "--privacy" {
			privacy = true
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fatalf("cannot determine working directory: %v", err)
	}
	if envBase := os.Getenv("CLAUDE_PROJECT_DIR"); envBase != "" {
		cwd = envBase
	}

	witnessDir := config.WitnessDir(cwd)
	cfg, err := config.Load(witnessDir)
	if err != nil {
		fatalf("load config: %v", err)
	}

	// Check if sync is configured and enabled.
	if !cfg.Sync.Enabled || cfg.Sync.ServerURL == "" {
		fmt.Println("sync disabled; set sync.server_url and sync.author_uuid in .witness/config.toml")
		fmt.Println()
		fmt.Println("Example config.toml snippet:")
		fmt.Println("  [sync]")
		fmt.Println("  enabled     = true")
		fmt.Println(`  server_url  = "http://your-server:8080"`)
		fmt.Println(`  author_uuid = "550e8400-e29b-41d4-a716-446655440000"`)
		return
	}

	if cfg.Sync.AuthorUUID == "" {
		fatalf("sync.author_uuid must be set in .witness/config.toml")
	}

	// Open the trace store.
	dbPath := filepath.Join(witnessDir, "trace.db")
	s, err := store.Open(dbPath)
	if err != nil {
		fatalf("open trace store: %v", err)
	}
	defer s.Close()

	// Honor privacy mode from config unless explicitly passed.
	if cfg.Privacy.Mode {
		privacy = true
	}

	cgpf.BinaryVersion = version

	doc, err := cgpf.Export(s, cgpf.ExportOptions{
		Privacy:  privacy,
		RepoPath: cwd,
	})
	if err != nil {
		fatalf("export: %v", err)
	}

	// Inject author_uuid into the document before marshalling.
	// The server validates this field; we attach it at the top level.
	type syncDoc struct {
		CGPFVersion string         `json:"cgpf_version"`
		GeneratedBy string         `json:"generated_by"`
		GeneratedAt string         `json:"generated_at"`
		AuthorUUID  string         `json:"author_uuid"`
		Repo        cgpf.RepoMeta  `json:"repo"`
		Sessions    []cgpf.Session `json:"sessions"`
	}

	payload := syncDoc{
		CGPFVersion: doc.CGPFVersion,
		GeneratedBy: doc.GeneratedBy,
		GeneratedAt: doc.GeneratedAt,
		AuthorUUID:  cfg.Sync.AuthorUUID,
		Repo:        doc.Repo,
		Sessions:    doc.Sessions,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		fatalf("marshal payload: %v", err)
	}

	endpoint := cfg.Sync.ServerURL + "/api/v1/ingest"
	fmt.Printf("syncing %d session(s) to %s ...\n", len(doc.Sessions), endpoint)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fatalf("HTTP POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		msg := errBody["error"]
		if msg == "" {
			msg = resp.Status
		}
		fatalf("server returned error: %s", msg)
	}

	fmt.Println("sync complete")
}
