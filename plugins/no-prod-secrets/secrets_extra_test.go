package main

import "testing"

// TestScanForSecrets_MultilineWithAWSKey verifies multiline text is scanned.
func TestScanForSecrets_MultilineWithAWSKey(t *testing.T) {
	diff := `+package main
+
+import "fmt"
+
+func main() {
+    key := "AKIAIOSFODNN7EXAMPLE"
+    fmt.Println(key)
+}
`
	if !ScanForSecrets(diff) {
		t.Error("expected true for AWS key in multiline diff")
	}
}

// TestScanForSecrets_NoMatchEmptyLines returns false for whitespace-only input.
func TestScanForSecrets_NoMatchEmptyLines(t *testing.T) {
	if ScanForSecrets("   \n\n\t\n") {
		t.Error("expected false for whitespace-only input")
	}
}

// TestScanForSecrets_BoundaryAWSKeyExactLength verifies 16 char AKIA suffix is required.
func TestScanForSecrets_BoundaryAWSKeyExactLength(t *testing.T) {
	// Exactly 16 uppercase alphanumeric chars after AKIA -- should match.
	exact := "AKIA1234567890ABCDEF"
	if !ScanForSecrets(exact) {
		t.Errorf("expected true for %q (exact 16 chars)", exact)
	}
	// 15 chars after AKIA -- should NOT match.
	short := "AKIA1234567890ABCD"
	if ScanForSecrets(short) {
		t.Errorf("expected false for %q (only 15 chars)", short)
	}
}

// TestScanForSecrets_SecretEqualSignVariants tests different spacing around =.
func TestScanForSecrets_SecretEqualSignVariants(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"space=space", `secret = "longpassword1234"`, true},
		{"no_spaces", `secret="longpassword1234"`, true},
		{"token_case_insensitive", `TOKEN="supersecretvalue"`, true},
		{"PASSWORD_caps", `PASSWORD="mysupersecretpw"`, true},
		{"value_too_short", `password="ab"`, false},
		{"unrelated_equals", `count=42`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScanForSecrets(tc.input)
			if got != tc.want {
				t.Errorf("ScanForSecrets(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestScanForSecrets_BothPatterns verifies both patterns can appear in the same input.
func TestScanForSecrets_BothPatterns(t *testing.T) {
	input := `AKIAIOSFODNN7EXAMPLE and secret="longpassword123"`
	if !ScanForSecrets(input) {
		t.Error("expected true for input containing both patterns")
	}
}

// TestScanForSecrets_DiffContext verifies detection in realistic diff context.
func TestScanForSecrets_DiffContext(t *testing.T) {
	diff := `@@ -1,5 +1,8 @@
 package config

+const (
+    awsKey    = "AKIAIOSFODNN7EXAMPLEX"
+    apiSecret = "supersecretvalue123"
+)

 func GetConfig() string {`
	if !ScanForSecrets(diff) {
		t.Error("expected true for AWS key in realistic diff")
	}
}

// TestScanForSecrets_SingleQuoteSecret verifies single-quote credential detection.
func TestScanForSecrets_SingleQuoteSecret(t *testing.T) {
	input := `api_key = 'very_long_api_key_here_exceeds_8'`
	if !ScanForSecrets(input) {
		t.Error("expected true for single-quote api_key assignment")
	}
}
