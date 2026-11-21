// Package lang tokenizes Python and C-like sources and scores cognitive complexity.
package lang

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Token is a lexical unit with source offsets. Comments and whitespace are omitted.
type Token struct {
	Kind  string
	Text  string
	Line  int
	Start int
	End   int
}

// Function is one analyzed function or method.
type Function struct {
	Name       string
	StartLine  int
	EndLine    int
	Cognitive  int
	MaxNesting int
	Lines      int
}

// Language returns the policy language id for a repo-relative path.
func Language(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return "python", true
	case ".ts", ".tsx":
		return "typescript", true
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript", true
	case ".go":
		return "go", true
	case ".java":
		return "java", true
	case ".kt", ".kts":
		return "kotlin", true
	case ".rs":
		return "rust", true
	default:
		return "", false
	}
}

// Supported reports whether Analyze can score this language.
func Supported(language string) bool {
	switch language {
	case "python", "typescript", "javascript", "go":
		return true
	default:
		return false
	}
}

// Analyze returns functions in source. Unbalanced syntax returns an error.
func Analyze(language, source string) ([]Function, error) {
	switch language {
	case "python":
		return analyzePython(source)
	case "typescript", "javascript":
		return analyzeC(source, cConfig{funcKw: []string{"function"}, arrows: true})
	case "go":
		return analyzeC(source, cConfig{funcKw: []string{"func"}, arrows: false})
	default:
		return nil, fmt.Errorf("no analyzer for %s", language)
	}
}

// Tokens returns comment-free tokens for a full file or a diff hunk.
// A fragment that does not lex returns an error so callers can fail closed.
func Tokens(language, source string) ([]Token, error) {
	switch language {
	case "python":
		return lexPython(source)
	case "typescript", "javascript", "go":
		return lexC(source)
	default:
		return nil, fmt.Errorf("no tokenizer for %s", language)
	}
}

// IsTest reports paths that hold tests or human oracle fixtures.
func IsTest(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") || strings.HasSuffix(base, "_test.go") {
		return true
	}
	if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}
	p := "/" + strings.ToLower(filepath.ToSlash(path))
	for _, s := range []string{"/test/", "/tests/", "/oracle/", "/__tests__/"} {
		if strings.Contains(p, s) {
			return true
		}
	}
	return false
}
