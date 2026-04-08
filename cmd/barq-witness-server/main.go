// barq-witness-server is the self-hosted team aggregator for barq-witness.
// It accepts anonymized CGPF v0.3 exports from multiple developers, stores
// summary statistics, and serves a simple HTML team dashboard.
//
// Usage:
//
//	barq-witness-server [--port <port>] [--db <path>]
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yasserrmd/barq-witness/internal/server"
)

const defaultPort = 8080
const defaultDB = "./witness-server.db"

func main() {
	port := defaultPort
	dbPath := defaultDB

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--port" && i+1 < len(args):
			i++
			p, err := strconv.Atoi(args[i])
			if err != nil {
				fatalf("invalid --port value %q: %v", args[i], err)
			}
			port = p
		case strings.HasPrefix(args[i], "--port="):
			val := strings.TrimPrefix(args[i], "--port=")
			p, err := strconv.Atoi(val)
			if err != nil {
				fatalf("invalid --port value %q: %v", val, err)
			}
			port = p
		case args[i] == "--db" && i+1 < len(args):
			i++
			dbPath = args[i]
		case strings.HasPrefix(args[i], "--db="):
			dbPath = strings.TrimPrefix(args[i], "--db=")
		case args[i] == "--help" || args[i] == "-h":
			printUsage()
			os.Exit(0)
		default:
			fatalf("unknown flag: %s", args[i])
		}
	}

	srv, err := server.New(dbPath, port)
	if err != nil {
		fatalf("init server: %v", err)
	}

	fmt.Printf("barq-witness-server listening on :%d (db: %s)\n", port, dbPath)
	if err := srv.Start(); err != nil {
		fatalf("server error: %v", err)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `barq-witness-server -- self-hosted team aggregator

Usage:
  barq-witness-server [flags]

Flags:
  --port <port>   HTTP listen port (default: %d)
  --db   <path>   SQLite database path (default: %s)
  --help          show this help

Endpoints:
  POST /api/v1/ingest     ingest a CGPF v0.3 export
  GET  /api/v1/stats      JSON aggregate stats
  GET  /api/v1/dashboard  HTML team dashboard
`, defaultPort, defaultDB)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "barq-witness-server: "+format+"\n", args...)
	os.Exit(1)
}
