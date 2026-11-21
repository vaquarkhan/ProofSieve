package gitx

import (
	"context"
	"os/exec"
	"testing"
)

func TestShowBadRev(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	_, ok, err := Show(context.Background(), repo, "bad", "a.txt")
	if ok || err == nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
