package pathutil

import (
	"path/filepath"
	"testing"
)

func TestRel(t *testing.T) {
	repo := t.TempDir()
	got, err := Rel(repo, "a/b.py")
	if err != nil || got != "a/b.py" {
		t.Fatalf("rel=%q %v", got, err)
	}
	got, err = Rel(repo, filepath.Join(repo, "a", "c.py"))
	if err != nil || got != "a/c.py" {
		t.Fatalf("abs=%q %v", got, err)
	}
	if _, err = Rel(repo, ""); err == nil {
		t.Fatal("empty")
	}
	if _, err = Rel(repo, ".."); err == nil {
		t.Fatal("escape")
	}
	if _, err = Rel(repo, "."); err == nil {
		t.Fatal("root")
	}
	outside := filepath.Clean(filepath.Join(repo, "..", "outside.txt"))
	if _, err = Rel(repo, outside); err == nil {
		t.Fatal("outside abs")
	}
}
