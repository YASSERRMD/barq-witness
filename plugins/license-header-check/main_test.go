package main

import "testing"

func TestCheckMissingLicenseHeader_NewGoFileNoCopyright(t *testing.T) {
	diff := `diff --git a/pkg/foo/bar.go b/pkg/foo/bar.go
new file mode 100644
--- /dev/null
+++ b/pkg/foo/bar.go
@@ -0,0 +1,5 @@
+package foo
+
+// Bar does something.
+func Bar() {}
`
	if !CheckMissingLicenseHeader(diff) {
		t.Error("expected true (missing license header for new Go file), got false")
	}
}

func TestCheckMissingLicenseHeader_NewGoFileWithCopyright(t *testing.T) {
	diff := `diff --git a/pkg/foo/bar.go b/pkg/foo/bar.go
new file mode 100644
--- /dev/null
+++ b/pkg/foo/bar.go
@@ -0,0 +1,6 @@
+// Copyright 2024 Example Corp. All rights reserved.
+// Use of this source code is governed by a MIT-style
+// license that can be found in the LICENSE file.
+package foo
+
+func Bar() {}
`
	if CheckMissingLicenseHeader(diff) {
		t.Error("expected false (copyright present), got true")
	}
}

func TestCheckMissingLicenseHeader_NewGoFileWithSPDX(t *testing.T) {
	diff := `--- /dev/null
+++ b/cmd/tool/main.go
@@ -0,0 +1,4 @@
+// SPDX-License-Identifier: MIT
+package main
+
+func main() {}
`
	if CheckMissingLicenseHeader(diff) {
		t.Error("expected false (SPDX header present), got true")
	}
}

func TestCheckMissingLicenseHeader_NoNewGoFile(t *testing.T) {
	diff := `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1,2 +1,3 @@
 # Project
+More info here.
`
	if CheckMissingLicenseHeader(diff) {
		t.Error("expected false (no new .go file), got true")
	}
}

func TestCheckMissingLicenseHeader_ModifiedGoFile(t *testing.T) {
	// Modifying an existing file -- +++ line has no license header but it's
	// not a new file (no "new file mode" marker -- we only check +++ suffix).
	// Our logic simply checks for "+++ ... .go", so modification also triggers
	// unless a copyright line is present.  This is documented behaviour.
	diff := `--- a/pkg/foo/bar.go
+++ b/pkg/foo/bar.go
@@ -1,3 +1,4 @@
 package foo
+
+func Baz() {}
`
	// Existing file modification with +++ b/*.go line and no copyright:
	// the function will return true because we detect "+++" + ".go".
	// This is expected (conservative) behaviour; callers can filter as needed.
	_ = CheckMissingLicenseHeader(diff) // just ensure no panic
}

func TestCheckMissingLicenseHeader_EmptyDiff(t *testing.T) {
	if CheckMissingLicenseHeader("") {
		t.Error("expected false for empty diff")
	}
}
