package analyzer

import (
	"path/filepath"
	"strings"
)

// securityGlobs is the list of glob patterns that indicate a security-sensitive
// file path.  All matches are case-insensitive.
var securityGlobs = []string{
	"**/auth/**",
	"**/oauth/**",
	"**/login/**",
	"**/session/**",
	"**/token*",
	"**/jwt*",
	"**/password*",
	"**/secret*",
	"**/credential*",
	"**/crypto/**",
	"**/encrypt*",
	"**/payment/**",
	"**/billing/**",
	"**/checkout/**",
	"**/wallet/**",
	"**/admin/**",
	"**/sudo/**",
	"**/permission*",
	"**/rbac/**",
	"**/.env*",
	"**/config/secrets*",
}

// depFiles is the set of well-known dependency manifest file names.
var depFiles = map[string]bool{
	"package.json":    true,
	"go.mod":          true,
	"requirements.txt": true,
	"cargo.toml":      true,
	"pyproject.toml":  true,
	"gemfile":         true,
}

// IsSecurityPath returns true if filePath matches any security-sensitive glob.
// The comparison is case-insensitive and uses forward-slash separators.
func IsSecurityPath(filePath string) bool {
	normalized := strings.ToLower(filepath.ToSlash(filePath))
	for _, pattern := range securityGlobs {
		matched, err := filepath.Match(strings.ToLower(pattern), normalized)
		if err == nil && matched {
			return true
		}
		// filepath.Match does not support **, so we also try a contains-based
		// approach for the path-segment prefix.
		if matchDoubleStarGlob(pattern, normalized) {
			return true
		}
	}
	return false
}

// IsDependencyFile returns true if filePath is a known dependency manifest.
func IsDependencyFile(filePath string) bool {
	base := strings.ToLower(filepath.Base(filePath))
	return depFiles[base]
}

// matchDoubleStarGlob handles simple **/<suffix> and **/<prefix>* patterns
// by stripping the **/ prefix and checking if any path component matches.
func matchDoubleStarGlob(pattern, path string) bool {
	pattern = strings.ToLower(pattern)

	// Strip leading **/
	stripped := strings.TrimPrefix(pattern, "**/")
	if stripped == pattern {
		return false // no ** to expand
	}

	// If the remaining pattern still contains /, split on path separators.
	if !strings.Contains(stripped, "/") {
		// Match against any single path component or suffix.
		if strings.HasSuffix(stripped, "/**") {
			dir := strings.TrimSuffix(stripped, "/**")
			return strings.Contains(path, "/"+dir+"/") ||
				strings.HasPrefix(path, dir+"/")
		}
		// Check if the path contains a segment that matches (glob-style).
		segments := strings.Split(path, "/")
		for _, seg := range segments {
			if ok, _ := filepath.Match(stripped, seg); ok {
				return true
			}
		}
		// Also check as suffix of the full path.
		if ok, _ := filepath.Match("*/"+stripped, path); ok {
			return true
		}
		if ok, _ := filepath.Match(stripped, filepath.Base(path)); ok {
			return true
		}
		return false
	}

	// Pattern has a slash -- check if path contains the sub-path.
	if strings.HasSuffix(stripped, "/**") {
		dir := strings.TrimSuffix(stripped, "/**")
		return strings.Contains(path, "/"+dir+"/") ||
			strings.HasPrefix(path, dir+"/")
	}

	return strings.Contains(path, stripped)
}
