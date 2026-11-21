package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Session{}).Save(filepath.Join(file, "session.json")); err == nil {
		t.Fatal("mkdir")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "session.json")
	if err := os.MkdirAll(target+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (Session{Files: map[string]int{}}).Save(target); err == nil {
		t.Fatal("write")
	}
	orig := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("marshal") }
	t.Cleanup(func() { marshalIndent = orig })
	if err := (Session{}).Save(filepath.Join(t.TempDir(), "s.json")); err == nil {
		t.Fatal("marshal")
	}
	if _, err := Load(target+".tmp", "h", time.Now()); err == nil {
		t.Fatal("load directory")
	}
}
