package guard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
)

func writeLock(t *testing.T, repo string) {
	t.Helper()
	lock, err := lockfile.Generate(lockfile.Options{
		Repo: repo, Spec: "specs/a.md", ChangeType: "bugfix",
		Allow: []string{"billing/**"}, Deny: []string{"billing/secret/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lockfile.Save(filepath.Join(repo, ".proofsieve", "scope.lock.yaml"), lock); err != nil {
		t.Fatal(err)
	}
}

func TestAllowAndDeny(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "billing"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeLock(t, repo)
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	var out, err bytes.Buffer
	payload := `{"tool_name":"Write","cwd":"` + jsonPath(repo) + `","tool_input":{"path":"billing/a.py","contents":"x = 1\n"}}`
	code := Run(strings.NewReader(payload), &out, &err, Options{Now: now})
	if code != 0 || !strings.Contains(out.String(), `"permission":"allow"`) && !strings.Contains(out.String(), `"permission": "allow"`) {
		t.Fatalf("code %d out %s err %s", code, out.String(), err.String())
	}
	out.Reset()
	err.Reset()
	bad := `{"tool_name":"Write","tool_input":{"file_path":"auth/a.py","contents":"x"}}`
	code = Run(strings.NewReader(bad), &out, &err, Options{Repo: repo, Now: now})
	if code != 2 || !strings.Contains(err.String(), "outside-scope") {
		t.Fatalf("%d %s", code, err.String())
	}
	out.Reset()
	err.Reset()
	policy := `{"tool_name":"Edit","tool_input":{"path":".proofsieve/policy.yaml","old_string":"a","new_string":"b"}}`
	if Run(strings.NewReader(policy), &out, &err, Options{Repo: repo, Now: now}) != 2 {
		t.Fatal(err.String())
	}
	out.Reset()
	err.Reset()
	shell := `{"tool_name":"Shell","tool_input":{"command":"git apply .proofsieve/scope.lock.yaml"}}`
	if Run(strings.NewReader(shell), &out, &err, Options{Repo: repo, Now: now}) != 2 {
		t.Fatal("shell", err.String())
	}
	out.Reset()
	err.Reset()
	okShell := `{"command":"go test ./billing"}`
	if Run(strings.NewReader(okShell), &out, &err, Options{Repo: repo, Now: now}) != 0 {
		t.Fatal("ok shell", err.String())
	}
}

func TestPayloadErrors(t *testing.T) {
	var out, err bytes.Buffer
	if Run(strings.NewReader("nope"), &out, &err, Options{}) != 2 {
		t.Fatal("json")
	}
	out.Reset()
	err.Reset()
	if Run(badReader{}, &out, &err, Options{}) != 2 {
		t.Fatal("read")
	}
	out.Reset()
	err.Reset()
	orig := getwd
	getwd = func() (string, error) { return "", os.ErrClosed }
	defer func() { getwd = orig }()
	if Run(strings.NewReader(`{"tool_name":"Shell","tool_input":{"command":"go test"}}`), &out, &err, Options{}) != 2 {
		t.Fatal("getwd")
	}
	repo := t.TempDir()
	writeLock(t, repo)
	out.Reset()
	err.Reset()
	if Run(strings.NewReader(`{"tool_name":"Write","tool_input":{}}`), &out, &err, Options{Repo: repo}) != 2 {
		t.Fatal("no path")
	}
	out.Reset()
	err.Reset()
	if Run(strings.NewReader(`{"tool_name":"Write","tool_input":{"path":"../outside.txt"}}`), &out, &err, Options{Repo: repo}) != 2 {
		t.Fatal("escape")
	}
	out.Reset()
	err.Reset()
	if Run(strings.NewReader(`{"tool_name":"Delete","tool_input":{"path":"billing/a.py"}}`), &out, &err, Options{Repo: repo}) != 0 {
		t.Fatal("delete", err.String())
	}
	out.Reset()
	err.Reset()
	secret := `{"tool_name":"Shell","tool_input":{"command":"cat billing/secret/keys"}}`
	if Run(strings.NewReader(secret), &out, &err, Options{Repo: repo}) != 2 {
		t.Fatal("secret", err.String())
	}
	if Run(strings.NewReader(`{}`), &out, &err, Options{Repo: filepath.Join(repo, "missing-lock")}) != 2 {
		t.Fatal("lock")
	}
}

type badReader struct{}

func (badReader) Read([]byte) (int, error) { return 0, os.ErrClosed }

func jsonPath(path string) string {
	return strings.ReplaceAll(path, "\\", "\\\\")
}
