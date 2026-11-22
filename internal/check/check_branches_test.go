package check

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/diffparse"
	"github.com/vaquarkhan/ProofSieve/internal/lang"
	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
	"github.com/vaquarkhan/ProofSieve/internal/policy"
)

func TestInfraEdges(t *testing.T) {
	origGet := getwd
	getwd = func() (string, error) { return "", errors.New("no wd") }
	t.Cleanup(func() { getwd = origGet })
	if _, code := Run(context.Background(), Options{}); code != 2 {
		t.Fatal("getwd")
	}
	getwd = func() (string, error) { return t.TempDir(), nil }
	if _, code := Run(context.Background(), Options{Gate: ""}); code != 2 {
		t.Fatal("missing policy from cwd")
	}
	repo := initRepo(t)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy(""))
	if _, code := Run(context.Background(), Options{Repo: repo, Base: "HEAD", LockPath: filepath.Join(repo, "no.lock")}); code != 2 {
		t.Fatal("lock")
	}
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("bugfix", 6, 400, 15, `["**"]`, `[]`, ""))
	base := commitAll(t, repo, "base")
	origDiff := diffFn
	t.Cleanup(func() { diffFn = origDiff })
	diffFn = func(context.Context, string, string, string) (string, error) { return "", errors.New("diff") }
	if _, code := Run(context.Background(), Options{Repo: repo, Base: base}); code != 2 {
		t.Fatal("diff")
	}
	diffFn = func(context.Context, string, string, string) (string, error) {
		return "diff --git a/a.py b/a.py\n@@ bad\n", nil
	}
	if _, code := Run(context.Background(), Options{Repo: repo, Base: base}); code != 2 {
		t.Fatal("parse")
	}
	diffFn = origDiff
	origShow := showFn
	showFn = func(context.Context, string, string, string) ([]byte, bool, error) {
		return nil, false, errors.New("show")
	}
	t.Cleanup(func() { showFn = origShow })
	if _, code := Run(context.Background(), Options{Repo: repo, Base: base}); code != 2 {
		t.Fatal("show")
	}
}

func TestScopeComplexityChurn(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := initRepo(t)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("java"))
	writeRel(t, repo, "specs/a.md", "REQ-1 holds the boundary\n")
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x\n")
	writeRel(t, repo, "notes/a.md", "note\n")
	writeRel(t, repo, "src/A.java", "class A {}\n")
	base := commitAll(t, repo, "base")
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("bugfix", 1, 1, 15, `["billing/**"]`, `[".proofsieve/**"]`, ""))
	writeRel(t, repo, "billing/price.py", "def price(x):\n        return x\n")
	writeRel(t, repo, "billing/other.py", "def other(x):\n    return x\n")
	writeRel(t, repo, "notes/a.md", "note changed\n")
	writeRel(t, repo, "src/A.java", "class A { int x; }\n")
	commitAll(t, repo, "head")
	rep, code := Run(context.Background(), Options{Repo: repo, Base: base, Gate: "scope"})
	if code != 1 || !hasRule(rep, "max-files") || !hasRule(rep, "max-loc") || !hasRule(rep, "churn") {
		t.Fatalf("scope %d %s", code, rep.JSON())
	}
	_, _ = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "complexity"})

	repo = initRepo(t)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy(""))
	writeRel(t, repo, "specs/a.md", "REQ-1 holds the boundary\n")
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x\n")
	writeRel(t, repo, "billing/oracle/test_price.py", "def test_price():\n    assert True\n")
	base = commitAll(t, repo, "base")
	sum := shaOf(t, filepath.Join(repo, "billing", "oracle", "test_price.py"))
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("bugfix", 6, 400, 15, `["billing/**", "specs/**", ".proofsieve/**"]`, `["auth/**"]`, "    - path: billing/missing.py\n      sha256: "+sum+"\n    - path: billing/oracle/test_price.py\n      sha256: 0000000000000000000000000000000000000000000000000000000000000000\n"))
	writeRel(t, repo, "billing/price.py", "def price(x):\n    if x:\n        if x:\n            if x:\n                if x:\n                    if x:\n                        return x\n    return 0\n")
	writeRel(t, repo, "billing/new.py", "def created(x):\n    return x\n")
	writeRel(t, repo, "billing/gone.py", "def gone(x):\n    return x\n")
	writeRel(t, repo, "auth/no.py", "x = 1\n")
	writeRel(t, repo, "billing/oracle/test_price.py", "def test_price():\n    assert True\n")
	os.Remove(filepath.Join(repo, "billing", "gone.py"))
	commitAll(t, repo, "head")
	os.Remove(filepath.Join(repo, "billing", "price.py"))
	rep, code = Run(context.Background(), Options{Repo: repo, Base: base})
	if code != 1 {
		t.Fatalf("want findings %d %s", code, rep.JSON())
	}

	repo = initRepo(t)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy(""))
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x\n")
	writeRel(t, repo, "lib/main.go", "package p\nfunc F() int { return 1 }\n")
	base = commitAll(t, repo, "base")
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("feature", 6, 400, 15, `["**"]`, `[]`, ""))
	writeRel(t, repo, "billing/bad.py", "def bad():\n    return 'nope\n")
	writeRel(t, repo, "lib/main.go", "package p\nfunc F() int { return 2 }\n")
	commitAll(t, repo, "head")
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "complexity"}); code != 1 {
		t.Fatal("language and parse", code)
	}
}

func TestMutationAndReview(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := initRepo(t)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
mutation_gate:
  enabled: true
  test_command: []
`)
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x > 0\n")
	base := commitAll(t, repo, "base")
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("feature", 6, 400, 15, `["**"]`, `[]`, ""))
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x > 1\n")
	commitAll(t, repo, "head")
	if _, code := Run(context.Background(), Options{Repo: repo, Base: base, Gate: "mutation"}); code != 2 {
		t.Fatal("empty command")
	}

	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
mutation_gate:
  enabled: true
  test_command: ["test"]
  max_mutants: 1
  min_tier1_kill_rate_pct: 80
  min_tier2_kill_rate_pct: 70
`)
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x > 1 and x > 2 and x > 3\n")
	commitAll(t, repo, "more")
	rep, code := Run(context.Background(), Options{Repo: repo, Base: base, Gate: "mutation", MutateExec: func(context.Context, string, []string) (int, error) {
		return 0, nil
	}})
	if code != 1 || !hasRule(rep, "mutation-score") {
		t.Fatalf("score %d %s", code, rep.JSON())
	}

	writeRel(t, repo, "billing/price.py", "flag = True\n")
	commitAll(t, repo, "bool")
	rep, _ = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "mutation", MutateExec: func(context.Context, string, []string) (int, error) {
		return 0, nil
	}})
	_ = rep

	os.Remove(filepath.Join(repo, "billing", "price.py"))
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "mutation"}); code != 2 {
		t.Fatal("missing source")
	}

	repo = initRepo(t)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
mutation_gate:
  enabled: true
  test_command: ["test"]
review_triage:
  enabled: true
  tier0_sarif_paths: [".proofsieve/missing.sarif"]
  test_command: ["test"]
comprehension_gate:
  enabled: true
  risk_threshold: 1
  critical_paths: [".."]
  require_structured_attestation: true
  require_diff_quiz: true
`)
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x\n")
	writeRel(t, repo, "specs/a.md", "other\n")
	base = commitAll(t, repo, "base")
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("feature", 6, 400, 15, `["**"]`, `[]`, ""))
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x > 0\n")
	commitAll(t, repo, "head")
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, MutateExec: func(context.Context, string, []string) (int, error) {
		return 0, errors.New("exec")
	}}); code != 2 {
		t.Fatal("mutate exec")
	}

	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
mutation_gate:
  enabled: false
review_triage:
  enabled: true
  tier0_sarif_paths: [".proofsieve/a.sarif"]
  test_command: ["test"]
  ignore_registry: .proofsieve/ignore.json
comprehension_gate:
  enabled: true
  risk_threshold: 1
  critical_paths: ["billing/**"]
  require_structured_attestation: true
  require_diff_quiz: false
`)
	writeRel(t, repo, ".proofsieve/a.sarif", `{"runs":[{"results":[{"ruleId":"r","level":"note","message":{"text":"n"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"billing/price.py"},"region":{"startLine":1}}}]}]}]}`)
	writeRel(t, repo, ".proofsieve/review.json", `[{"claim":"unsafe","test":"a|b","expect":{"base":"pass","head":"fail"}},{"claim":"real bug here","test":"test_price","expect":{"base":"pass","head":"fail"},"severity":"warning","rule":"bug","path":"billing/price.py","line":1}]`)
	writeRel(t, repo, ".proofsieve/ignore.json", `{"fingerprints":[]}`)
	commitAll(t, repo, "review")
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, ReviewExec: func(context.Context, string, []string) (int, error) {
		return 0, errors.New("repro")
	}}); code != 2 {
		t.Fatal("repro")
	}
	rep, code = Run(context.Background(), Options{Repo: repo, Base: base, ReviewExec: func(context.Context, string, []string) (int, error) {
		return 1, nil
	}})
	if code == 2 {
		t.Fatalf("review infra %s", rep.JSON())
	}
	writeRel(t, repo, ".proofsieve/review.json", `{`)
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base}); code != 2 {
		t.Fatal("review json")
	}
	os.Remove(filepath.Join(repo, ".proofsieve", "review.json"))
	os.Mkdir(filepath.Join(repo, ".proofsieve", "review.json"), 0o755)
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base}); code != 2 {
		t.Fatal("review dir")
	}
	os.Remove(filepath.Join(repo, ".proofsieve", "review.json"))
	writeRel(t, repo, ".proofsieve/a.sarif", `{`)
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base}); code != 2 {
		t.Fatal("sarif")
	}
	writeRel(t, repo, ".proofsieve/ignore.json", `{`)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
review_triage:
  enabled: true
  tier0_sarif_paths: []
  test_command: []
  ignore_registry: .proofsieve/ignore.json
`)
	writeRel(t, repo, ".proofsieve/review.json", `[{"claim":"needs a test","test":"test_price","expect":{"base":"pass","head":"fail"}}]`)
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base}); code != 2 {
		t.Fatal("empty review command or ignore")
	}

	sha := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
comprehension_gate:
  enabled: true
  risk_threshold: 1
  critical_paths: ["billing/**"]
  require_structured_attestation: true
`)
	writeRel(t, repo, ".proofsieve/ignore.json", `{`)
	os.Remove(filepath.Join(repo, ".proofsieve", "review.json"))
	comment := "/proofsieve attest\nsha: " + sha + "\nspec: REQ-1\ninvariant: price stays explicit\nfailure: revert the price commit\ntest: test_missing\n"
	Run(context.Background(), Options{Repo: repo, Base: base, AttestText: "nope", AttestAuthor: "dev"})
	Run(context.Background(), Options{Repo: repo, Base: base, AttestText: comment, AttestAuthor: "ci[bot]"})
	Run(context.Background(), Options{Repo: repo, Base: base, AttestText: comment, AttestAuthor: "dev"})
	writeRel(t, repo, "specs/a.md", "REQ-1 holds the boundary\n")
	Run(context.Background(), Options{Repo: repo, Base: base, AttestText: "/proofsieve attest\nsha: " + sha + "\nspec: specs/a.md\ninvariant: price stays explicit\nfailure: revert the price commit\ntest: test_price\n", AttestAuthor: "dev"})
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "ownership", AttestText: comment, AttestAuthor: "dev"}); code == 2 {
		t.Fatal("ownership infra")
	}
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "none"}); code != 0 {
		t.Fatal("unknown gate")
	}
}

func hasRule(rep interface{ JSON() string }, rule string) bool {
	return strings.Contains(rep.JSON(), rule)
}

func pythonPolicy(extra string) string {
	langs := "[python]"
	if extra != "" {
		langs = "[python, " + extra + "]"
	}
	return "version: \"1.0\"\nlanguages: " + langs + "\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\n"
}

func lockYAML(change string, files, loc, churn int, allow, deny, locked string) string {
	return "version: \"1.0\"\nspec_ref: specs/a.md\nchange_type: " + change + "\nowner: dev\nblast_radius:\n  max_files: " + itoa(files) + "\n  max_loc_delta: " + itoa(loc) + "\n  max_nonfunctional_churn_pct: " + itoa(churn) + "\n  allowed_paths: " + allow + "\n  denied_paths: " + deny + "\ntest_integrity:\n  allow_test_deletion: false\n  allow_skip_annotations: false\n  allow_test_mutation: false\n  locked_files:\n" + locked
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.email", "dev@example.com")
	git(t, repo, "config", "user.name", "ProofSieve Tests")
	git(t, repo, "config", "commit.gpgsign", "false")
	git(t, repo, "config", "core.autocrlf", "false")
	return repo
}

func writeRel(t *testing.T, repo, rel, body string) {
	t.Helper()
	full := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, repo, msg string) string {
	t.Helper()
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-m", msg, "--allow-empty")
	return strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
}

func TestRemainingBranches(t *testing.T) {
	repo := initRepo(t)
	body := "def keep():\n    return 1\n\nx = 1\n"
	var big strings.Builder
	big.WriteString("def big(x):\n")
	for i := 0; i < 60; i++ {
		big.WriteString("    x = x\n")
	}
	big.WriteString("    return x\n")
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("java"))
	writeRel(t, repo, "specs/a.md", "REQ-1 holds the boundary\n")
	writeRel(t, repo, "billing/price.py", body)
	writeRel(t, repo, "billing/old.py", "def old():\n    return 1\n")
	writeRel(t, repo, "billing/oracle/test_price.py", "def test_price():\n    assert True\n")
	writeRel(t, repo, "notes/a.md", "note\n")
	writeRel(t, repo, "src/A.java", "class A {}\n")
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("bugfix", 8, 400, 15, `["**"]`, `[]`, ""))
	base := commitAll(t, repo, "base")
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("java")+"# edited\n")
	writeRel(t, repo, "billing/price.py", "def keep():\n    return 1\n\nx = 2\n")
	writeRel(t, repo, "billing/big.py", big.String())
	writeRel(t, repo, "billing/oracle/test_price.py", "def test_price():\n    pass\n")
	writeRel(t, repo, "notes/a.md", "note 2\n")
	writeRel(t, repo, "src/A.java", "class A { int x; }\n")
	git(t, repo, "rm", "billing/old.py")
	commitAll(t, repo, "head")
	os.Remove(filepath.Join(repo, "billing", "oracle", "test_price.py"))
	rep, code := Run(context.Background(), Options{Repo: repo, Base: base})
	if code != 1 || !hasRule(rep, "policy-path") || !hasRule(rep, "complexity") {
		t.Fatalf("remaining %d %s", code, rep.JSON())
	}

	repo = initRepo(t)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
mutation_gate:
  enabled: true
  test_command: ["test"]
review_triage:
  enabled: true
  tier0_sarif_paths: [".proofsieve/missing.sarif"]
comprehension_gate:
  enabled: true
  risk_threshold: 1
  critical_paths: ["auth/**"]
  require_diff_quiz: true
  require_structured_attestation: false
`)
	writeRel(t, repo, "billing/price.py", "def price(x):\n    return x\n")
	writeRel(t, repo, "billing/oracle/test_price.py", "def test_price():\n    assert True\n")
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("feature", 6, 400, 15, `["**"]`, `[]`, ""))
	base = commitAll(t, repo, "base")
	writeRel(t, repo, "billing/price.py", "def price(x):\n    if x > 0:\n        return eval(x)\n    return 0\n")
	writeRel(t, repo, "billing/oracle/test_price.py", "def test_price():\n    assert 1 == 1\n")
	commitAll(t, repo, "head")
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "mutation", MutateExec: func(context.Context, string, []string) (int, error) {
		return 1, nil
	}}); code != 0 && code != 1 {
		t.Fatal("mutation gaps", code)
	}
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "review"}); code != 2 {
		t.Fatal("missing sarif")
	}
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "ownership"}); code != 1 {
		t.Fatal("quiz")
	}

	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
review_triage:
  enabled: true
  tier0_sarif_paths: []
  test_command: ["test"]
  ignore_registry: .proofsieve/ignore.json
comprehension_gate:
  enabled: true
  risk_threshold: 90
  critical_paths: [".."]
  require_structured_attestation: true
`)
	os.MkdirAll(filepath.Join(repo, ".proofsieve", "ignore.json"), 0o755)
	if _, code = Run(context.Background(), Options{Repo: repo, Base: base, Gate: "review"}); code != 2 {
		t.Fatal("ignore dir")
	}
	os.RemoveAll(filepath.Join(repo, ".proofsieve", "ignore.json"))
	plain := initRepo(t)
	writeRel(t, plain, ".proofsieve/policy.yaml", pythonPolicy(""))
	writeRel(t, plain, "notes/a.md", "note\n")
	writeRel(t, plain, "src/A.java", "class A {}\n")
	writeRel(t, plain, ".proofsieve/scope.lock.yaml", lockYAML("bugfix", 6, 400, 15, `["**"]`, `[]`, ""))
	plainBase := commitAll(t, plain, "base")
	writeRel(t, plain, "notes/a.md", "note 2\n")
	writeRel(t, plain, "src/A.java", "class B {}\n")
	commitAll(t, plain, "head")
	if _, code = Run(context.Background(), Options{Repo: plain, Base: plainBase, Gate: "scope"}); code == 2 {
		t.Fatal("plain churn")
	}
	writeRel(t, repo, ".proofsieve/a.sarif", `{"runs":[{"results":[{"ruleId":"r","level":"error","message":{"text":"boom"},"locations":[]}]}]}`)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
review_triage:
  enabled: true
  tier0_sarif_paths: [".proofsieve/a.sarif"]
  test_command: ["test"]
comprehension_gate:
  enabled: true
  critical_paths: [".."]
`)
	n := 0
	rep, code = Run(context.Background(), Options{Repo: repo, Base: base, ReviewExec: func(context.Context, string, []string) (int, error) {
		n++
		if n%2 == 1 {
			return 0, nil
		}
		return 1, nil
	}})
	if code == 2 {
		t.Fatal(rep.JSON())
	}
	sha := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	orig := readFile
	readFile = func(string) ([]byte, error) { return nil, errors.New("unreadable") }
	t.Cleanup(func() { readFile = orig })
	comment := "/proofsieve attest\nsha: " + sha + "\nspec: REQ-1\ninvariant: price stays explicit\nfailure: revert the price commit\ntest: test_price\n"
	writeRel(t, repo, "specs/a.md", "REQ-1 holds the boundary\n")
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("feature", 6, 400, 15, `["**"]`, `[]`, ""))
	Run(context.Background(), Options{Repo: repo, Base: base, Gate: "ownership", AttestText: comment, AttestAuthor: "dev"})
}

func TestDirectGaps(t *testing.T) {
	repo := t.TempDir()
	writeRel(t, repo, "a.py", "def keep():\n    return 1\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\nx = 1\n")
	pol, err := policy.Parse([]byte(pythonPolicy("")))
	if err != nil {
		t.Fatal(err)
	}
	file := diffparse.File{OldPath: "a.py", NewPath: "a.py", Hunks: []diffparse.Hunk{{NewStart: 20, NewLines: 1, NewText: "x = 2\n"}}}
	_ = complexityFindings(Options{Repo: repo}, pol, []diffparse.File{file})
	if overlaps(lang.Function{StartLine: 1, EndLine: 2}, map[int]bool{4: true}) {
		t.Fatal("overlap")
	}
	orig := showFn
	showFn = func(context.Context, string, string, string) ([]byte, bool, error) {
		return nil, false, errors.New("show")
	}
	t.Cleanup(func() { showFn = orig })
	out := scopeFindings(context.Background(), Options{Repo: repo}, lockfile.Lock{}, []diffparse.File{{
		OldPath: "billing/oracle/test_a.py", NewPath: "billing/oracle/test_a.py",
		Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 1, NewText: "def test_a():\n"}},
	}}, false, true)
	if len(out) == 0 {
		t.Fatal("show error finding")
	}
}
