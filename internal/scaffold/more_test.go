package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInitEdges(t *testing.T) {
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("dist"), 0o644)
	if _, err := Init(repo); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, []byte("x"), 0o644)
	if _, err := Init(filepath.Join(file, "child")); err == nil {
		t.Fatal("parent")
	}
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".gitignore"), 0o755)
	if _, err := Init(dir); err == nil {
		t.Fatal("gitignore dir")
	}
	cursor := t.TempDir()
	os.WriteFile(filepath.Join(cursor, ".cursor"), []byte("x"), 0o644)
	if _, err := Init(cursor); err == nil {
		t.Fatal("cursor file")
	}
	claude := t.TempDir()
	os.WriteFile(filepath.Join(claude, ".claude"), []byte("x"), 0o644)
	if _, err := Init(claude); err == nil {
		t.Fatal("claude file")
	}
	if runtime.GOOS == "windows" {
		if err := writeNew(filepath.Join(t.TempDir(), "a|b"), "x", &Result{}); err == nil {
			t.Fatal("stat")
		}
	}
	origStat := statPath
	statPath = func(string) (os.FileInfo, error) { return nil, errors.New("stat") }
	if err := writeNew(filepath.Join(t.TempDir(), "stat.txt"), "x", &Result{}); err == nil {
		t.Fatal("stat")
	}
	statPath = origStat
	worig := writeFile
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	t.Cleanup(func() { writeFile = worig })
	if err := writeNew(filepath.Join(t.TempDir(), "a.txt"), "x", &Result{}); err == nil {
		t.Fatal("write")
	}
	if err := ensureIgnore(t.TempDir(), &Result{}); err == nil {
		t.Fatal("ignore write")
	}
	writeFile = worig
	orig := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("json") }
	t.Cleanup(func() { marshalIndent = orig })
	if _, err := Init(t.TempDir()); err == nil {
		t.Fatal("marshal")
	}
}
