// Package attest parses and checks a human ownership comment.
package attest

import (
	"fmt"
	"strings"
	"unicode"
)

// Attestation is the three-field ownership record plus the commit and spec id.
type Attestation struct {
	SHA       string
	Spec      string
	Invariant string
	Failure   string
	Test      string
}

// Parse reads a /proofsieve attest comment.
func Parse(text string) (Attestation, error) {
	var a Attestation
	seen := map[string]bool{}
	found := false
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.Contains(trim, "/proofsieve attest") {
			found = true
			continue
		}
		key, val, ok := strings.Cut(trim, ":")
		if !ok {
			return Attestation{}, fmt.Errorf("attestation line %q needs a colon", trim)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "sha":
			a.SHA = val
		case "spec":
			a.Spec = val
		case "invariant":
			a.Invariant = val
		case "failure":
			a.Failure = val
		case "test":
			a.Test = val
		default:
			return Attestation{}, fmt.Errorf("unknown attestation field %q", key)
		}
		seen[key] = true
	}
	if !found {
		return Attestation{}, fmt.Errorf("attestation must contain /proofsieve attest")
	}
	for _, key := range []string{"sha", "spec", "invariant", "failure", "test"} {
		if !seen[key] {
			return Attestation{}, fmt.Errorf("attestation is missing %s", key)
		}
	}
	if len(a.Invariant) < 10 || len(a.Failure) < 10 {
		return Attestation{}, fmt.Errorf("invariant and failure must each be at least 10 characters")
	}
	return a, nil
}

// SpecID returns the requirement id from a spec field.
func SpecID(spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", fmt.Errorf("spec id is empty")
	}
	if i := strings.LastIndex(spec, "#"); i >= 0 {
		spec = spec[i+1:]
	}
	if spec == "" || strings.Contains(spec, "/") || strings.HasSuffix(spec, ".md") {
		return "", fmt.Errorf("spec field must name an id, for example REQ-1")
	}
	return spec, nil
}

// HoldsSpec reports whether id appears in the spec text.
func HoldsSpec(specText, id string) bool {
	return id != "" && strings.Contains(specText, id)
}

// HoldsTest reports whether name is an identifier in source and the line is not skipped.
func HoldsTest(source, name string) bool {
	if name == "" {
		return false
	}
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if !identOnLine(line, name) {
			continue
		}
		if skippedLine(line) || (i+1 < len(lines) && skippedLine(lines[i+1])) {
			return false
		}
		return true
	}
	return false
}

func skippedLine(line string) bool {
	return strings.Contains(line, "skip") || strings.Contains(line, "Skip") || strings.Contains(line, "@Disabled")
}

func identOnLine(line, name string) bool {
	for i := 0; i+len(name) <= len(line); i++ {
		if line[i:i+len(name)] != name {
			continue
		}
		if i > 0 && isIdent(rune(line[i-1])) {
			continue
		}
		if i+len(name) < len(line) && isIdent(rune(line[i+len(name)])) {
			continue
		}
		return true
	}
	return false
}

func isIdent(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// AuthorOK reports whether login is a human account.
func AuthorOK(login string) bool {
	login = strings.TrimSpace(login)
	if login == "" {
		return false
	}
	return !strings.Contains(strings.ToLower(login), "[bot]")
}
