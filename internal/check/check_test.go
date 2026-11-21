package check

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v %s", args, err, out)
	}
}

func TestRunPassAndFail(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := t.TempDir()
	ctx := context.Background()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "dev@example.com")
	git(t, repo, "config", "user.name", "ProofSieve Tests")
	git(t, repo, "config", "commit.gpgsign", "false")
	must := func(rel, body string) {
		t.Helper()
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(".proofsieve/policy.yaml", "version: \"1.0\"\nlanguages: [python]\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\n")
	must(".proofsieve/scope.lock.yaml", "version: \"1.0\"\nspec_ref: specs/a.md\nchange_type: bugfix\nowner: dev\nblast_radius:\n  max_files: 6\n  max_loc_delta: 250\n  max_nonfunctional_churn_pct: 15\n  allowed_paths: [\"billing/**\", \"specs/**\"]\n  denied_paths: [\".proofsieve/**\"]\ntest_integrity:\n  allow_test_deletion: false\n  allow_skip_annotations: false\n  allow_test_mutation: false\n  locked_files: []\n")
	must("specs/a.md", "REQ-1 rounds half up\n")
	must("billing/price.py", "def price(x):\n    return x\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	must("billing/price.py", "def price(x):\n    if x > 0:\n        return x\n    return 0\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "head")
	rep, code := Run(ctx, Options{Repo: repo, Base: base, Head: "HEAD"})
	if code != 0 {
		t.Fatalf("code %d %s", code, rep.JSON())
	}
	must("auth/a.py", "x = 1\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "wide")
	rep, code = Run(ctx, Options{Repo: repo, Base: base, Head: "HEAD", Gate: "scope"})
	if code != 1 || !rep.Failed() {
		t.Fatalf("expected scope failure %d %s", code, rep.JSON())
	}
	if _, code = Run(ctx, Options{Repo: repo, Base: base, Head: "no-such", Gate: "scope"}); code != 2 {
		t.Fatal("missing rev")
	}
	if _, code = Run(ctx, Options{Repo: repo, PolicyPath: filepath.Join(repo, "missing.yaml"), Base: base, Head: "HEAD"}); code != 2 {
		t.Fatal("policy")
	}
}

func TestOwnershipAndMutation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := t.TempDir()
	ctx := context.Background()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "dev@example.com")
	git(t, repo, "config", "user.name", "ProofSieve Tests")
	git(t, repo, "config", "commit.gpgsign", "false")
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("specs/a.md", "REQ-1 auth boundary\n")
	write("billing/oracle/test_auth.py", "def test_auth():\n    assert True\n")
	write("auth/login.py", "def login(x):\n    return x\n")
	write(".proofsieve/policy.yaml", "version: \"1.0\"\nlanguages: [python]\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nmutation_gate:\n  enabled: true\n  test_command: [\"test\"]\ncomprehension_gate:\n  enabled: true\n  risk_threshold: 30\n  critical_paths: [\"auth/**\"]\n  require_structured_attestation: true\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	sum := shaOf(t, filepath.Join(repo, "billing", "oracle", "test_auth.py"))
	write(".proofsieve/scope.lock.yaml", "version: \"1.0\"\nspec_ref: specs/a.md\nchange_type: feature\nowner: dev\nblast_radius:\n  max_files: 6\n  max_loc_delta: 250\n  allowed_paths: [\"auth/**\", \"billing/**\", \"specs/**\", \".proofsieve/**\"]\n  denied_paths: []\ntest_integrity:\n  allow_test_deletion: false\n  allow_skip_annotations: false\n  allow_test_mutation: false\n  locked_files:\n    - path: billing/oracle/test_auth.py\n      sha256: "+sum+"\n")
	write("auth/login.py", "def login(x):\n    if x > 0:\n        return x\n    return 0\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "head")
	sha := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	rep, code := Run(ctx, Options{Repo: repo, Base: base, Head: "HEAD", MutateExec: func(context.Context, string, []string) (int, error) {
		return 1, nil
	}})
	if code == 0 {
		t.Fatalf("expected block before attest %s", rep.JSON())
	}
	comment := "/proofsieve attest\nsha: " + sha + "\nspec: REQ-1\ninvariant: login stays explicit\nfailure: revert the auth commit\ntest: test_auth\n"
	rep, code = Run(ctx, Options{Repo: repo, Base: base, Head: "HEAD", AttestText: comment, AttestAuthor: "dev", MutateExec: func(context.Context, string, []string) (int, error) {
		return 1, nil
	}})
	if code != 0 {
		t.Fatalf("attest should pass %d %s", code, rep.JSON())
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

func shaOf(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
