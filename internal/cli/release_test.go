package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/minsign"
)

func TestKeygenSignAndTelemetry(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "minisign.pub")
	sec := filepath.Join(dir, "minisign.key")
	var out, errb bytes.Buffer
	if Execute([]string{"keygen"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("usage")
	}
	if Execute([]string{"keygen", "--public", pub, "--secret", sec}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	if Execute([]string{"verify-evidence"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("verify usage")
	}
	body := []byte("{\"sha\":\"abc\"}\n")
	ev := filepath.Join(dir, "evidence.json")
	os.WriteFile(ev, body, 0o644)
	if err := signEvidence(sec, body, ev+".minisig"); err != nil {
		t.Fatal(err)
	}
	if Execute([]string{"verify-evidence", "--evidence", ev, "--public", pub, "--signature", ev + ".minisig"}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatal(errb.String())
	}
	os.WriteFile(ev, []byte("changed\n"), 0o644)
	if Execute([]string{"verify-evidence", "--evidence", ev, "--public", pub, "--signature", ev + ".minisig"}, strings.NewReader(""), &out, &errb) != 1 {
		t.Fatal("tamper")
	}
	if err := signEvidence(filepath.Join(dir, "missing.key"), body, ev+".minisig"); err == nil {
		t.Fatal("missing key")
	}
	os.WriteFile(filepath.Join(dir, "bad.key"), []byte("nope\n"), 0o644)
	if err := signEvidence(filepath.Join(dir, "bad.key"), body, ev+".minisig"); err == nil {
		t.Fatal("bad key")
	}
	if Execute([]string{"check", "--base", "HEAD", "--sign-key", sec}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("sign without evidence")
	}
	os.Mkdir(filepath.Join(dir, "secdir"), 0o755)
	if Execute([]string{"keygen", "--public", filepath.Join(dir, "again.pub"), "--secret", filepath.Join(dir, "secdir")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("secret dir")
	}
	os.Mkdir(filepath.Join(dir, "pubdir"), 0o755)
	if Execute([]string{"keygen", "--public", filepath.Join(dir, "pubdir"), "--secret", filepath.Join(dir, "sec2")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("public dir")
	}
	if Execute([]string{"verify-evidence", "--evidence", filepath.Join(dir, "nope"), "--public", pub, "--signature", ev + ".minisig"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("missing evidence")
	}
	if Execute([]string{"verify-evidence", "--evidence", ev, "--public", filepath.Join(dir, "no-pub"), "--signature", ev + ".minisig"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("missing public")
	}
	if Execute([]string{"verify-evidence", "--evidence", ev, "--public", pub, "--signature", filepath.Join(dir, "no.sig")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("missing sig")
	}
	origSign := signMessage
	signMessage = func(minsign.Key, []byte, string) (string, error) { return "", errors.New("sign") }
	if err := signEvidence(sec, body, ev+".minisig"); err == nil {
		t.Fatal("sign")
	}
	signMessage = origSign
	origKey := newKey
	newKey = func() (minsign.Key, error) { return minsign.Key{}, errors.New("key") }
	if Execute([]string{"keygen", "--public", filepath.Join(dir, "p2"), "--secret", filepath.Join(dir, "s2")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("generate")
	}
	newKey = origKey
	signMessage = func(minsign.Key, []byte, string) (string, error) { return "", errors.New("sign") }
	if Execute([]string{"check", "--repo", dir, "--base", "HEAD", "--evidence", filepath.Join(dir, "out.json"), "--sign-key", sec}, strings.NewReader(""), &out, &errb) == 0 {
		t.Fatal("sign during check")
	}
	signMessage = origSign

	repo := gitRepo(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	evidence := filepath.Join(repo, "evidence.json")
	if Execute([]string{"check", "--repo", repo, "--base", "HEAD", "--evidence", evidence, "--sign-key", sec, "--otel-endpoint", srv.URL}, strings.NewReader(""), &out, &errb) != 0 {
		t.Fatalf("pass %s", errb.String())
	}
	if _, err := os.Stat(evidence + ".minisig"); err != nil {
		t.Fatal(err)
	}
	if Execute([]string{"check", "--repo", repo, "--base", "HEAD", "--otel-endpoint", "http://127.0.0.1:1"}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("otel down")
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "dev@example.com")
	run("config", "user.name", "ProofSieve Tests")
	run("config", "commit.gpgsign", "false")
	run("config", "core.autocrlf", "false")
	os.MkdirAll(filepath.Join(repo, ".proofsieve"), 0o755)
	os.WriteFile(filepath.Join(repo, ".proofsieve", "policy.yaml"), []byte("version: \"1.0\"\nlanguages: [go]\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\n"), 0o644)
	os.WriteFile(filepath.Join(repo, ".proofsieve", "scope.lock.yaml"), []byte("version: \"1.0\"\nspec_ref: specs/a.md\nchange_type: feature\nowner: dev\nblast_radius:\n  max_files: 6\n  max_loc_delta: 250\n  allowed_paths: [\"**\"]\n  denied_paths: [\".proofsieve/**\"]\ntest_integrity:\n  locked_files: []\n"), 0o644)
	os.MkdirAll(filepath.Join(repo, "specs"), 0o755)
	os.WriteFile(filepath.Join(repo, "specs", "a.md"), []byte("REQ-1\n"), 0o644)
	run("add", "-A")
	run("commit", "-m", "base")
	return repo
}
