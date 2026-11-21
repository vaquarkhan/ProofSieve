package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	ctx := context.Background()
	if _, err := Run(ctx, repo, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, repo, "config", "user.email", "dev@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, repo, "config", "user.name", "ProofSieve Tests"); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, repo, "config", "commit.gpgsign", "false"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "a.txt")
	if err := os.WriteFile(path, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, repo, "add", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, repo, "commit", "-m", "base"); err != nil {
		t.Fatal(err)
	}
	base, err := Rev(ctx, repo, "HEAD")
	if err != nil || len(base) < 7 {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, repo, "add", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, repo, "commit", "-m", "head"); err != nil {
		t.Fatal(err)
	}
	diff, err := Diff(ctx, repo, base, "HEAD")
	if err != nil || diff == "" {
		t.Fatal(diff, err)
	}
	body, ok, err := Show(ctx, repo, base, "a.txt")
	if err != nil || !ok || string(body) != "one\n" {
		t.Fatalf("%q %v %v", body, ok, err)
	}
	_, ok, err = Show(ctx, repo, base, "missing.txt")
	if err != nil || ok {
		t.Fatalf("missing %v %v", ok, err)
	}
	dest := filepath.Join(t.TempDir(), "wt")
	if err := AddWorktree(ctx, repo, dest, base); err != nil {
		t.Fatal(err)
	}
	if err := RemoveWorktree(ctx, repo, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := Rev(ctx, repo, "no-such-ref"); err == nil {
		t.Fatal("wanted missing ref")
	}
	if _, err := Run(ctx, repo, "not-a-git-command"); err == nil {
		t.Fatal("wanted git error")
	}
}
