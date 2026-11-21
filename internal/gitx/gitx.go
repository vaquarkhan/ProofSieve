// Package gitx runs git without a shell.
package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes git in repo and returns stdout.
func Run(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if repo != "" {
		cmd.Dir = repo
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Diff returns a unified diff for base...head.
func Diff(ctx context.Context, repo, base, head string) (string, error) {
	return Run(ctx, repo, "diff", "-U0", "-M", base+"..."+head)
}

// Rev parses a revision to a commit sha.
func Rev(ctx context.Context, repo, ref string) (string, error) {
	out, err := Run(ctx, repo, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Show returns file bytes at rev. ok is false when the path is absent.
func Show(ctx context.Context, repo, rev, path string) ([]byte, bool, error) {
	out, err := Run(ctx, repo, "show", rev+":"+path)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "exists on disk") || strings.Contains(msg, "Path") {
			return nil, false, nil
		}
		return nil, false, err
	}
	return []byte(out), true, nil
}

// AddWorktree checks out rev into dest.
func AddWorktree(ctx context.Context, repo, dest, rev string) error {
	_, err := Run(ctx, repo, "worktree", "add", "--detach", "--force", dest, rev)
	return err
}

// RemoveWorktree deletes a worktree.
func RemoveWorktree(ctx context.Context, repo, dest string) error {
	_, err := Run(ctx, repo, "worktree", "remove", "--force", dest)
	return err
}
