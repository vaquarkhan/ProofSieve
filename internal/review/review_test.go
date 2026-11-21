package review

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectAndSARIF(t *testing.T) {
	claims := []Claim{
		{Claim: "real", Test: "TestA", Expect: Expectation{Base: "pass", Head: "fail"}, Severity: "warning", Rule: "bug", Path: "a.py", Line: 1},
		{Claim: "shell", Test: "rm -rf /; echo", Expect: Expectation{Base: "pass", Head: "fail"}},
		{Claim: "ignored", Test: "TestB", Expect: Expectation{Base: "pass", Head: "fail"}, Rule: "nit"},
		{Claim: "second", Test: "TestC", Expect: Expectation{Base: "pass", Head: "fail"}, Severity: "error"},
	}
	if !Unsafe(claims[1].Test) || Unsafe("TestA") {
		t.Fatal("unsafe")
	}
	if !Match(claims[0].Expect, true, false) || Match(Expectation{}, true, false) {
		t.Fatal("match")
	}
	ignore := map[string]bool{Fingerprint(claims[2]): true}
	proved := map[int]bool{0: true, 1: true, 2: true, 3: true}
	kept := Select(claims, ignore, proved, 5)
	if len(kept) != 2 || kept[0].Severity != "error" {
		t.Fatalf("%+v", kept)
	}
	if len(Select(claims, ignore, proved, 1)) != 1 {
		t.Fatal("cap")
	}
	parsed, err := ParseSARIF([]byte(`{"runs":[{"results":[{"ruleId":"r","level":"error","message":{"text":"boom"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.py"},"region":{"startLine":3}}}]}]}]}`))
	if err != nil || parsed[0].Path != "a.py" || parsed[0].Severity != "error" {
		t.Fatalf("%+v %v", parsed, err)
	}
	if _, err := ParseSARIF([]byte("{")); err == nil {
		t.Fatal("bad sarif")
	}
	warn, err := ParseSARIF([]byte(`{"runs":[{"results":[{"ruleId":"w","level":"warning","message":{"text":"w"}},{"ruleId":"n","level":"note","message":{"text":"n"}}]}]}`))
	if err != nil || warn[0].Severity != "warning" || warn[1].Severity != "info" {
		t.Fatal(warn)
	}
	ig, err := LoadIgnore([]byte(`{"fingerprints":["a"]}`))
	if err != nil || !ig["a"] {
		t.Fatal(ig)
	}
	if _, err := LoadIgnore([]byte("{")); err == nil {
		t.Fatal("bad ignore")
	}
	empty, err := LoadIgnore(nil)
	if err != nil || len(empty) != 0 {
		t.Fatal(empty)
	}
	if _, err := DockerArgs("", "/tmp", []string{"pytest"}); err == nil {
		t.Fatal("image")
	}
	args, err := DockerArgs("python:3.12", "/work", []string{"pytest", "TestA"})
	if err != nil || args[0] != "run" || args[len(args)-1] != "TestA" {
		t.Fatal(args)
	}
}

func TestIgnoreFileAndReproduce(t *testing.T) {
	dir := t.TempDir()
	missing, err := ReadIgnoreFile(filepath.Join(dir, "nope.json"))
	if err != nil || len(missing) != 0 {
		t.Fatal(missing, err)
	}
	path := filepath.Join(dir, "ignore.json")
	if err := os.WriteFile(path, []byte(`{"fingerprints":["x"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIgnoreFile(path)
	if err != nil || !got["x"] {
		t.Fatal(got, err)
	}
	if _, err := ReadIgnoreFile(dir); err == nil {
		t.Fatal("dir")
	}
	if _, _, err := Reproduce(context.Background(), dir, "a", "b", "rm -rf", nil, nil); err == nil {
		t.Fatal("unsafe")
	}
	if _, _, err := Reproduce(context.Background(), dir, "a", "b", "TestA", nil, nil); err == nil {
		t.Fatal("no runner")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := t.TempDir()
	run := func(ctx context.Context, dir string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v %s", args, err, out)
		}
	}
	ctx := context.Background()
	run(ctx, repo, "init", "-b", "main")
	run(ctx, repo, "config", "core.autocrlf", "false")
	run(ctx, repo, "config", "user.email", "dev@example.com")
	run(ctx, repo, "config", "user.name", "ProofSieve Tests")
	run(ctx, repo, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(ctx, repo, "add", "a.txt")
	run(ctx, repo, "commit", "-m", "base")
	baseCmd := exec.Command("git", "rev-parse", "HEAD")
	baseCmd.Dir = repo
	baseOut, err := baseCmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	base := string(baseOut)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(ctx, repo, "add", "a.txt")
	run(ctx, repo, "commit", "-m", "head")
	execer := func(ctx context.Context, dir string, command []string) (int, error) {
		b, err := os.ReadFile(filepath.Join(dir, "a.txt"))
		if err != nil {
			return 1, err
		}
		if strings.TrimSpace(string(b)) == "one" {
			return 0, nil
		}
		return 1, nil
	}
	basePass, headPass, err := Reproduce(ctx, repo, strings.TrimSpace(base), "HEAD", "TestA", []string{"check"}, execer)
	if err != nil || !basePass || headPass {
		t.Fatalf("%v %v %v", basePass, headPass, err)
	}
	bad := func(context.Context, string, []string) (int, error) { return 0, os.ErrClosed }
	if _, _, err := Reproduce(ctx, repo, "HEAD", "HEAD", "TestA", []string{"check"}, bad); err == nil {
		t.Fatal("exec")
	}
}
