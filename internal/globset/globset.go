// Package globset matches repo-relative slash paths against scope globs.
package globset

import (
	"fmt"
	"strings"
)

// Set is a compiled list of globs. An empty set matches nothing.
type Set struct {
	raw []string
}

// Compile rejects empty patterns and any pattern that contains "..".
func Compile(patterns []string) (Set, error) {
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
		if p == "" {
			return Set{}, fmt.Errorf("empty glob")
		}
		if strings.Contains(p, "..") {
			return Set{}, fmt.Errorf("glob %q must not contain ..", p)
		}
		out = append(out, p)
	}
	return Set{raw: out}, nil
}

// Match reports whether path matches any pattern.
func (s Set) Match(path string) bool {
	path = clean(path)
	for _, p := range s.raw {
		if match(p, path) {
			return true
		}
	}
	return false
}

// Patterns returns a copy of the compiled patterns.
func (s Set) Patterns() []string {
	return append([]string(nil), s.raw...)
}

func clean(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = strings.TrimPrefix(path, "./")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return strings.TrimPrefix(path, "/")
}

func match(pattern, path string) bool {
	return matchParts(split(pattern), split(path))
}

func split(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func matchParts(pat, path []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(path); i++ {
				if matchParts(pat[1:], path[i:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		if !matchSeg(pat[0], path[0]) {
			return false
		}
		pat = pat[1:]
		path = path[1:]
	}
	return len(path) == 0
}

func matchSeg(pattern, seg string) bool {
	return matchWild([]rune(pattern), []rune(seg))
}

func matchWild(pat, seg []rune) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case '*':
			if matchWild(pat[1:], seg) {
				return true
			}
			if len(seg) == 0 {
				return false
			}
			seg = seg[1:]
		case '?':
			if len(seg) == 0 {
				return false
			}
			pat = pat[1:]
			seg = seg[1:]
		default:
			if len(seg) == 0 || seg[0] != pat[0] {
				return false
			}
			pat = pat[1:]
			seg = seg[1:]
		}
	}
	return len(seg) == 0
}
