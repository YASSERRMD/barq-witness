// Package util provides shared helper functions for barq-witness.
package util

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hex returns the lowercase hex-encoded SHA-256 digest of data.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA256HexString returns the lowercase hex-encoded SHA-256 digest of s.
func SHA256HexString(s string) string {
	return SHA256Hex([]byte(s))
}
