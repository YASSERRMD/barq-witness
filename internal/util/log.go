package util

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// OpenLogger returns a *log.Logger that appends to
// <witnessDir>/barq-witness.log, creating the file if necessary.
// If the file cannot be opened the logger falls back to io.Discard so
// that the caller never has to handle a nil pointer.
func OpenLogger(witnessDir string) *log.Logger {
	logPath := filepath.Join(witnessDir, "barq-witness.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return log.New(io.Discard, "", 0)
	}
	return log.New(f, "", log.LstdFlags|log.Lmicroseconds)
}
