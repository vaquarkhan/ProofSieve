package testgen

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripEdges(t *testing.T) {
	if _, err := Build("spec", map[string]string{"notes.md": "hello"}); err == nil {
		t.Fatal("language")
	}
	src := "# return value\ndef f():\n    return value\n"
	if _, err := Build("spec", map[string]string{"a.py": src}); err == nil {
		t.Fatal("leak")
	}
	goSrc := "package p\nfunc F() int {\n    return 1\n}\n"
	if _, err := Build("spec", map[string]string{"a.go": goSrc}); err != nil {
		t.Fatal(err)
	}
	if _, err := Build("spec", map[string]string{"a.py": "def f(:\n    return 'nope\n"}); err == nil {
		t.Fatal("parse")
	}
	dir := filepath.Join(t.TempDir(), "file")
	os.WriteFile(dir, []byte("x"), 0o644)
	if err := Write(filepath.Join(dir, "out"), Bundle{}); err == nil {
		t.Fatal("mkdir")
	}
	out := t.TempDir()
	os.Mkdir(filepath.Join(out, "bundle.json"), 0o755)
	if err := Write(out, Bundle{Spec: "s"}); err == nil {
		t.Fatal("bundle")
	}
	if _, _, err := strip("go", "package p\nfunc F() {}\n"); err != nil {
		t.Fatal(err)
	}
	orig := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("json") }
	t.Cleanup(func() { marshalIndent = orig })
	if err := Write(t.TempDir(), Bundle{Spec: "s"}); err == nil {
		t.Fatal("marshal")
	}
	if !leaked("return value", []string{"return value"}) || leaked("pass", []string{"pass"}) {
		t.Fatal("leaked")
	}
	if !strings.Contains(goSrc, "func") {
		t.Fatal("src")
	}
}
