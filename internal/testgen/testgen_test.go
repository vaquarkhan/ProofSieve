package testgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPython(t *testing.T) {
	src := "def round_half_up(amount, scale):\n    factor = 10 ** scale\n    return int(amount * factor + 0.5) / factor\n"
	b, err := Build("REQ-1 half up", map[string]string{"billing/rounding.py": src})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.Units[0].Source, "factor = 10") {
		t.Fatal(b.Units[0].Source)
	}
	if !strings.Contains(b.Units[0].Source, "def round_half_up") {
		t.Fatal(b.Units[0].Source)
	}
	dir := t.TempDir()
	if err := Write(dir, b); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bundle.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build("s", map[string]string{"README.md": "x"}); err == nil {
		t.Fatal("md")
	}
	if _, err := Build("s", map[string]string{"a.py": "def f():\n    return 'nope\n"}); err == nil {
		t.Fatal("parse")
	}
}

func TestLeak(t *testing.T) {
	if !leaked("factor = 10 ** scale", []string{"factor = 10 ** scale"}) {
		t.Fatal("leak")
	}
	if leaked("pass", []string{"pass"}) {
		t.Fatal("pass")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(filepath.Join(file, "out"), Bundle{}); err == nil {
		t.Fatal("mkdir")
	}
}
