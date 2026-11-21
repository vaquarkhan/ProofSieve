// Package pathutil turns tool and git paths into repo-relative slash paths.
package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Rel resolves candidate against repo and returns a clean slash path.
// Absolute paths must stay inside repo. Paths that escape with ".." are rejected.
func Rel(repo, candidate string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", fmt.Errorf("empty path")
	}
	repo = filepath.Clean(repo)
	var abs string
	if filepath.IsAbs(candidate) {
		abs = filepath.Clean(candidate)
	} else {
		abs = filepath.Clean(filepath.Join(repo, candidate))
	}
	rel, err := filepath.Rel(repo, abs)
	if err != nil {
		return "", fmt.Errorf("path %q: %w", candidate, err)
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("path %q escapes the repository", candidate)
	}
	if rel == "." {
		return "", fmt.Errorf("path names the repository root")
	}
	return rel, nil
}
