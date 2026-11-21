package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	repo := t.TempDir()
	res, err := Init(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 4 {
		t.Fatalf("%+v", res)
	}
	if !strings.Contains(Summary(res), "Created 4") {
		t.Fatal(Summary(res))
	}
	again, err := Init(repo)
	if err != nil || len(again.Kept) != 4 {
		t.Fatalf("%+v %v", again, err)
	}
	ignore := filepath.Join(repo, ".gitignore")
	if err := os.WriteFile(ignore, []byte("dist"), 0o644); err != nil {
		t.Fatal(err)
	}
	// file exists without the run entry and without a trailing newline
	third, err := Init(repo)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(ignore)
	if err != nil || !strings.Contains(string(b), ".proofsieve/run/") {
		t.Fatal(string(b), err)
	}
	_ = third
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(filepath.Join(file, "repo")); err == nil {
		t.Fatal("bad repo")
	}
}
