package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/review"
)

func TestExecuteSurface(t *testing.T) {
	var out, errb bytes.Buffer
	if Execute(nil, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("empty")
	}
	if Execute([]string{"nope"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("unknown")
	}
	out.Reset()
	if Execute([]string{"help"}, strings.NewReader(""), &out, &errb) != 0 || !strings.Contains(out.String(), "proofsieve guard") {
		t.Fatal(out.String())
	}
	out.Reset()
	if Execute([]string{"--help"}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal("help")
	}
	out.Reset()
	if Execute([]string{"version"}, strings.NewReader(""), &out, &errb) != 0 || !strings.Contains(out.String(), "1.0.0") {
		t.Fatal(out.String())
	}
	if Execute([]string{"init", "-bad"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("flag")
	}
	repo := t.TempDir()
	out.Reset()
	errb.Reset()
	if Execute([]string{"init", "--repo", repo}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Execute([]string{"init", "--repo", filepath.Join(parent, "child")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("init fail")
	}
	if Execute([]string{"lock", "--repo", repo}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("lock usage")
	}
	writeFile(t, filepath.Join(repo, "billing", "oracle", "test_a.py"), "def test_a():\n    assert True\n")
	out.Reset()
	errb.Reset()
	if Execute([]string{"lock", "--repo", repo, "--spec", "specs/a.md", "--allow", "billing/**", "--lock-glob", "missing/**"}, strings.NewReader(""), &out, &errb) != 1 {
		t.Fatal(errb.String())
	}
	writeFile(t, filepath.Join(repo, "specs", "a.md"), "REQ-1 holds\n")
	out.Reset()
	errb.Reset()
	if Execute([]string{"lock", "--repo", repo, "--spec", "specs/a.md", "--allow", "billing/**", "--lock-glob", "billing/oracle/**", "--owner", "dev", "--deny", "auth/**"}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
}

func TestGuardAttestAndVerify(t *testing.T) {
	repo := t.TempDir()
	var out, errb bytes.Buffer
	if Execute([]string{"init", "--repo", repo}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	writeFile(t, filepath.Join(repo, "billing", "oracle", "test_a.py"), "def test_a():\n    assert True\n")
	writeFile(t, filepath.Join(repo, "specs", "a.md"), "REQ-1 holds\n")
	if Execute([]string{"lock", "--repo", repo, "--spec", "specs/a.md", "--allow", "billing/**", "--lock-glob", "billing/oracle/**"}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	payload := `{"tool_name":"Write","tool_input":{"path":"auth/a.py","contents":"x=1\n"}}`
	out.Reset()
	errb.Reset()
	if Execute([]string{"guard"}, strings.NewReader(payload), &out, &errb) != 2 {
		t.Fatalf("guard %s %s", out.String(), errb.String())
	}
	out.Reset()
	errb.Reset()
	if Execute([]string{"attest", "--sha", "abc", "--spec", "REQ-1", "--invariant", "rounds half up", "--failure", "revert the pricing change", "--test", "test_a"}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	if Execute([]string{"attest", "--sha", "a", "--spec", "REQ-1", "--invariant", "short", "--failure", "revert the pricing change", "--test", "test_a"}, strings.NewReader(""), &out, &errb) != 1 {
		t.Fatal("short")
	}
	comment := filepath.Join(repo, "comment.txt")
	valid := "/proofsieve attest\nsha: abc\nspec: REQ-1\ninvariant: rounds half up\nfailure: revert the pricing change\ntest: test_a\n"
	writeFile(t, comment, valid)
	out.Reset()
	errb.Reset()
	args := []string{"attest", "verify", "--file", comment, "--sha", "abc", "--spec-file", filepath.Join(repo, "specs", "a.md"), "--repo", repo, "--author", "dev"}
	if Execute(args, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	args[len(args)-1] = "ci[bot]"
	if Execute(args, strings.NewReader(""), &out, &errb) != 1 {
		t.Fatal("bot")
	}
	if Execute([]string{"attest", "verify", "--file", filepath.Join(repo, "nope"), "--sha", "abc", "--spec-file", "x", "--author", "dev"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("missing")
	}
	bad := filepath.Join(repo, "bad.txt")
	writeFile(t, bad, "not an attestation\n")
	if Execute([]string{"attest", "verify", "--file", bad, "--sha", "abc", "--spec-file", filepath.Join(repo, "specs", "a.md"), "--author", "dev"}, strings.NewReader(""), &out, &errb) != 1 {
		t.Fatal("parse")
	}
	if Execute([]string{"attest", "verify", "--file", comment, "--sha", "zzz", "--spec-file", filepath.Join(repo, "specs", "a.md"), "--author", "dev"}, strings.NewReader(""), &out, &errb) != 1 {
		t.Fatal("sha")
	}
	if Execute([]string{"attest", "verify", "--file", comment, "--sha", "abc", "--spec-file", filepath.Join(repo, "missing.md"), "--author", "dev"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("spec file")
	}

	orig := reproduce
	defer func() { reproduce = orig }()
	reproduce = func(context.Context, string, string, string, string, []string, review.Exec) (bool, bool, error) {
		return true, false, nil
	}
	findings := filepath.Join(repo, "findings.json")
	writeFile(t, findings, `[{"claim":"bug","test":"test_a","expect":{"base":"pass","head":"fail"},"severity":"error","rule":"bug"}]`)
	kept := filepath.Join(repo, "kept.json")
	out.Reset()
	errb.Reset()
	if Execute([]string{"verify-findings", "--repo", repo, "--base", "HEAD", "--input", findings, "--out", kept}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	if Execute([]string{"verify-findings", "--input", findings}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("verify usage")
	}
	if Execute([]string{"verify-findings", "--base", "HEAD", "--input", filepath.Join(repo, "nope.json")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("verify missing")
	}
	writeFile(t, findings, `{`)
	if Execute([]string{"verify-findings", "--base", "HEAD", "--input", findings}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("verify json")
	}
	reproduce = func(context.Context, string, string, string, string, []string, review.Exec) (bool, bool, error) {
		return false, false, os.ErrClosed
	}
	writeFile(t, findings, `[{"claim":"bug","test":"test_a","expect":{"base":"pass","head":"fail"}}]`)
	if Execute([]string{"verify-findings", "--base", "HEAD", "--input", findings}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("verify exec")
	}
}

func TestCheckMutateTestGen(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := t.TempDir()
	gitc(t, repo, "init", "-b", "main")
	gitc(t, repo, "config", "user.email", "dev@example.com")
	gitc(t, repo, "config", "user.name", "ProofSieve Tests")
	gitc(t, repo, "config", "commit.gpgsign", "false")
	gitc(t, repo, "config", "core.autocrlf", "false")
	var out, errb bytes.Buffer
	if Execute([]string{"init", "--repo", repo}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	writeFile(t, filepath.Join(repo, "billing", "price.py"), "def price(x):\n    return x\n")
	writeFile(t, filepath.Join(repo, "specs", "a.md"), "REQ-1 price\n")
	if Execute([]string{"lock", "--repo", repo, "--spec", "specs/a.md", "--allow", "billing/**", "--allow", "specs/**"}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	gitc(t, repo, "add", ".")
	gitc(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(repo, "billing", "price.py"), "def price(x):\n    if x > 0:\n        return x\n    return 0\n")
	gitc(t, repo, "add", ".")
	gitc(t, repo, "commit", "-m", "head")
	evidence := filepath.Join(repo, "out", "evidence.json")
	out.Reset()
	errb.Reset()
	if code := Execute([]string{"check", "--repo", repo, "--base", base, "--format", "json", "--evidence", evidence}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("check %d %s %s", code, out.String(), errb.String())
	}
	out.Reset()
	if Execute([]string{"check", "--repo", repo, "--base", base, "--format", "sarif"}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	if Execute([]string{"check", "--repo", repo, "--base", base, "--format", "nope"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("format")
	}
	if Execute([]string{"check", "--repo", repo}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("base")
	}
	if Execute([]string{"check", "--repo", repo, "--base", base, "--attest-file", filepath.Join(repo, "missing.txt")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("attest file")
	}
	surv := filepath.Join(repo, "surv.json")
	_ = Execute([]string{"mutate", "--repo", repo, "--base", base, "--out", surv}, strings.NewReader(""), &out, &errb)
	if Execute([]string{"mutate"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("mutate usage")
	}
	gen := filepath.Join(repo, "gen")
	out.Reset()
	errb.Reset()
	if Execute([]string{"test-gen", "--repo", repo, "--spec", "specs/a.md", "--src", "billing/price.py", "--out", gen}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	if Execute([]string{"test-gen", "--repo", repo}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("usage")
	}
	if Execute([]string{"test-gen", "--repo", repo, "--spec", "missing.md", "--src", "billing/price.py", "--out", gen}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("spec")
	}
	if Execute([]string{"test-gen", "--repo", repo, "--spec", "specs/a.md", "--src", "missing.py", "--out", gen}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("src")
	}
	if Execute([]string{"-v"}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal("v")
	}
	multi := &multi{}
	if err := multi.Set("a"); err != nil || multi.String() != "a" {
		t.Fatal(multi)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitc(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v %s", args, err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
