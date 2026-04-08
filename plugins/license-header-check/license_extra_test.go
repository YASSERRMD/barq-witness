package main

import "testing"

// TestCheckMissingLicenseHeader_NoNewFiles returns false for a diff with no Go file additions.
func TestCheckMissingLicenseHeader_NoNewFiles(t *testing.T) {
	diff := `diff --git a/go.sum b/go.sum
--- a/go.sum
+++ b/go.sum
@@ -1,2 +1,3 @@
 existing line
+new dep v1.0.0 h1:abc
`
	if CheckMissingLicenseHeader(diff) {
		t.Error("expected false for diff with no new .go files")
	}
}

// TestCheckMissingLicenseHeader_MultipleNewGoFiles verifies any new file triggers check.
func TestCheckMissingLicenseHeader_MultipleNewGoFiles(t *testing.T) {
	diff := `--- /dev/null
+++ b/pkg/a/a.go
@@ -0,0 +1,3 @@
+package a
+func A() {}
--- /dev/null
+++ b/pkg/b/b.go
@@ -0,0 +1,3 @@
+package b
+func B() {}
`
	// Two new Go files, neither has a license header.
	if !CheckMissingLicenseHeader(diff) {
		t.Error("expected true for multiple new .go files with no license header")
	}
}

// TestCheckMissingLicenseHeader_NonGoNewFile returns false for new non-Go files.
func TestCheckMissingLicenseHeader_NonGoNewFile(t *testing.T) {
	diff := `--- /dev/null
+++ b/README.md
@@ -0,0 +1,3 @@
+# Project
+Description here.
`
	if CheckMissingLicenseHeader(diff) {
		t.Error("expected false for new non-Go file")
	}
}

// TestCheckMissingLicenseHeader_OnlyContextLines returns false for context-only diff.
func TestCheckMissingLicenseHeader_OnlyContextLines(t *testing.T) {
	diff := ` package main

 func main() {}
`
	if CheckMissingLicenseHeader(diff) {
		t.Error("expected false for context-only diff (no +++ lines)")
	}
}

// TestCheckMissingLicenseHeader_CopyrightInFirstLine verifies header on line 1.
func TestCheckMissingLicenseHeader_CopyrightInFirstLine(t *testing.T) {
	diff := `--- /dev/null
+++ b/pkg/foo/foo.go
@@ -0,0 +1,4 @@
+// Copyright 2024 Corp. All rights reserved.
+package foo
+func Foo() {}
`
	if CheckMissingLicenseHeader(diff) {
		t.Error("expected false for new Go file with Copyright on first line")
	}
}

// TestCheckMissingLicenseHeader_SPDXAtBottom verifies SPDX anywhere in diff is enough.
func TestCheckMissingLicenseHeader_SPDXAtBottom(t *testing.T) {
	diff := `--- /dev/null
+++ b/pkg/bar/bar.go
@@ -0,0 +1,5 @@
+package bar
+
+func Bar() {}
+
+// SPDX-License-Identifier: Apache-2.0
`
	if CheckMissingLicenseHeader(diff) {
		t.Error("expected false when SPDX present anywhere in diff")
	}
}

// TestCheckMissingLicenseHeader_EmptyString returns false for an empty string.
func TestCheckMissingLicenseHeader_EmptyString(t *testing.T) {
	if CheckMissingLicenseHeader("") {
		t.Error("expected false for empty string")
	}
}

// TestCheckMissingLicenseHeader_OnlyNewlineChars returns false for whitespace diff.
func TestCheckMissingLicenseHeader_OnlyNewlineChars(t *testing.T) {
	if CheckMissingLicenseHeader("\n\n\n") {
		t.Error("expected false for whitespace-only diff")
	}
}

// TestCheckMissingLicenseHeader_GoFileInSubdirectory handles nested path.
func TestCheckMissingLicenseHeader_GoFileInSubdirectory(t *testing.T) {
	diff := `--- /dev/null
+++ b/internal/pkg/subdir/deep.go
@@ -0,0 +1,3 @@
+package subdir
+func Deep() {}
`
	if !CheckMissingLicenseHeader(diff) {
		t.Error("expected true for new .go file in subdirectory with no license header")
	}
}

// TestCheckMissingLicenseHeader_TestFile verifies _test.go files are also checked.
func TestCheckMissingLicenseHeader_TestFile(t *testing.T) {
	diff := `--- /dev/null
+++ b/pkg/foo/foo_test.go
@@ -0,0 +1,5 @@
+package foo
+
+import "testing"
+
+func TestFoo(t *testing.T) {}
`
	if !CheckMissingLicenseHeader(diff) {
		t.Error("expected true for new _test.go file missing license header")
	}
}
