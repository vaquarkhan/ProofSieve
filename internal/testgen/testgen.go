// Package testgen builds a clean-room test context from a spec and signatures.
package testgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vaquarkhan/ProofSieve/internal/lang"
)

var marshalIndent = json.MarshalIndent

// Unit is one source file with implementation bodies removed.
type Unit struct {
	Path   string `json:"path"`
	Source string `json:"source"`
}

// Bundle is the context a test-writing agent is allowed to see.
type Bundle struct {
	Spec  string `json:"spec"`
	Units []Unit `json:"units"`
}

// Build strips implementation bodies. It returns an error if a body line would leak.
func Build(spec string, files map[string]string) (Bundle, error) {
	b := Bundle{Spec: spec}
	for path, src := range files {
		language, ok := lang.Language(path)
		if !ok || !lang.Supported(language) {
			return Bundle{}, fmt.Errorf("%s is not a supported source file", path)
		}
		stripped, bodies, err := strip(language, src)
		if err != nil {
			return Bundle{}, fmt.Errorf("%s: %w", path, err)
		}
		if leaked(stripped, bodies) {
			return Bundle{}, fmt.Errorf("%s: implementation body would be copied", path)
		}
		b.Units = append(b.Units, Unit{Path: path, Source: stripped})
	}
	return b, nil
}

func strip(language, src string) (string, []string, error) {
	fns, err := lang.Analyze(language, src)
	if err != nil {
		return "", nil, err
	}
	lines := strings.Split(src, "\n")
	var bodies []string
	for _, fn := range fns {
		limit := fn.EndLine
		if language != "python" {
			limit = fn.EndLine - 1
		}
		if limit <= fn.StartLine || fn.StartLine < 1 || fn.EndLine > len(lines) {
			continue
		}
		for i := fn.StartLine; i < limit && i < len(lines); i++ {
			bodies = append(bodies, strings.TrimSpace(lines[i]))
			if language == "python" {
				lines[i] = "    pass  # body omitted"
			} else {
				lines[i] = "  // body omitted"
			}
		}
	}
	return strings.Join(lines, "\n"), bodies, nil
}

func leaked(stripped string, bodies []string) bool {
	for _, line := range bodies {
		if len(line) < 8 || line == "pass" || strings.HasPrefix(line, "pass ") {
			continue
		}
		if strings.Contains(stripped, line) {
			return true
		}
	}
	return false
}

// Write saves bundle.json and bundle.md.
func Write(dir string, b Bundle) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := marshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	var md strings.Builder
	md.WriteString("# Clean-room context\n\n")
	md.WriteString(b.Spec)
	md.WriteString("\n")
	for _, unit := range b.Units {
		md.WriteString("\n## ")
		md.WriteString(unit.Path)
		md.WriteString("\n\n```\n")
		md.WriteString(unit.Source)
		md.WriteString("\n```\n")
	}
	return os.WriteFile(filepath.Join(dir, "bundle.md"), []byte(md.String()), 0o644)
}
