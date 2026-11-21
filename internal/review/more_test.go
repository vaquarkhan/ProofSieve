package review

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSelectAndReproduceEdges(t *testing.T) {
	if !Unsafe("") || !Unsafe("a;b") {
		t.Fatal("unsafe")
	}
	kept := Select([]Claim{{Claim: "x", Test: "t", Severity: ""}}, nil, map[int]bool{0: true}, 5)
	if len(kept) != 1 || kept[0].Severity != "error" {
		t.Fatal(kept)
	}
	if rank("info") != 2 {
		t.Fatal("rank")
	}
	if _, _, err := Reproduce(context.Background(), t.TempDir(), "HEAD", "HEAD", "", nil, nil); err == nil {
		t.Fatal("unsafe name")
	}
	orig := mkdirTemp
	mkdirTemp = func(string, string) (string, error) { return "", errors.New("temp") }
	t.Cleanup(func() { mkdirTemp = orig })
	if _, _, err := Reproduce(context.Background(), t.TempDir(), "HEAD", "HEAD", "test_a", []string{"go"}, func(context.Context, string, []string) (int, error) {
		return 0, nil
	}); err == nil {
		t.Fatal("temp")
	}
	n := 0
	mkdirTemp = func(dir, pattern string) (string, error) {
		n++
		if n == 1 {
			return orig(dir, pattern)
		}
		return "", errors.New("temp2")
	}
	if _, _, err := Reproduce(context.Background(), t.TempDir(), "HEAD", "HEAD", "test_a", []string{"go"}, func(context.Context, string, []string) (int, error) {
		return 0, nil
	}); err == nil {
		t.Fatal("temp2")
	}
	mkdirTemp = orig
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	if _, _, err := Reproduce(context.Background(), t.TempDir(), "HEAD", "HEAD", "test_a", []string{"go"}, func(context.Context, string, []string) (int, error) {
		return 0, errors.New("run")
	}); err == nil {
		t.Fatal("worktree or run")
	}
}

func TestReproduceGitEdges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v %s", args, err, out)
		}
	}
	git("init", "-b", "main")
	git("config", "user.email", "dev@example.com")
	git("config", "user.name", "ProofSieve Tests")
	git("config", "commit.gpgsign", "false")
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\n"), 0o644)
	git("add", ".")
	git("commit", "-m", "base")
	if _, _, err := Reproduce(context.Background(), repo, "deadbeef", "HEAD", "test_a", []string{"go"}, func(context.Context, string, []string) (int, error) {
		return 0, nil
	}); err == nil {
		t.Fatal("base")
	}
	if _, _, err := Reproduce(context.Background(), repo, "HEAD", "deadbeef", "test_a", []string{"go"}, func(context.Context, string, []string) (int, error) {
		return 0, nil
	}); err == nil {
		t.Fatal("head")
	}
	n := 0
	if _, _, err := Reproduce(context.Background(), repo, "HEAD", "HEAD", "test_a", []string{"go"}, func(context.Context, string, []string) (int, error) {
		n++
		if n == 1 {
			return 0, nil
		}
		return 0, errors.New("head")
	}); err == nil {
		t.Fatal("run")
	}
}
