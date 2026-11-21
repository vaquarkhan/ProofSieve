package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndNegativeCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	body := "version: \"1.0\"\nlanguages: [python]\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nmutation_gate:\n  max_mutants: -1\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("negative")
	}
	if _, err := Load(filepath.Join(path, "missing")); err == nil {
		t.Fatal("missing")
	}
}
