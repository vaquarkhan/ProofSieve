package guard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
)

func TestGuardEdges(t *testing.T) {
	orig := getwd
	getwd = func() (string, error) { return "", errors.New("wd") }
	t.Cleanup(func() { getwd = orig })
	if Run(strings.NewReader(`{}`), &strings.Builder{}, &strings.Builder{}, Options{}) != 2 {
		t.Fatal("wd")
	}
	getwd = func() (string, error) { return t.TempDir(), nil }
	if Run(strings.NewReader(`{}`), &strings.Builder{}, &strings.Builder{}, Options{}) != 2 {
		t.Fatal("wd ok")
	}
	getwd = orig
	repo := t.TempDir()
	lock := "version: \"1.0\"\nspec_ref: specs/a.md\nchange_type: bugfix\nblast_radius:\n  max_files: 2\n  max_loc_delta: 20\n  allowed_paths: [\"billing/**\"]\n  denied_paths: [\"auth/**\", \"a*b\"]\n"
	os.MkdirAll(filepath.Join(repo, ".proofsieve"), 0o755)
	os.WriteFile(filepath.Join(repo, ".proofsieve", "scope.lock.yaml"), []byte(lock), 0o644)
	os.MkdirAll(filepath.Join(repo, "billing"), 0o755)
	os.WriteFile(filepath.Join(repo, "billing", "a.py"), []byte("x = 1\n"), 0o644)
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	opts := Options{Repo: repo, Now: now}
	payload := `{"tool_name":"Write","tool_input":{"path":"billing/a.py","contents":"x = 2\ny = 3\n"}}`
	if Run(strings.NewReader(payload), &strings.Builder{}, &strings.Builder{}, opts) != 0 {
		t.Fatal("write")
	}
	if Run(strings.NewReader(payload), &strings.Builder{}, &strings.Builder{}, opts) != 0 {
		t.Fatal("replace")
	}
	edit := `{"tool_name":"Edit","tool_input":{"path":"billing/a.py","old_string":"x = 1","new_string":"x = 4"}}`
	if Run(strings.NewReader(edit), &strings.Builder{}, &strings.Builder{}, opts) != 0 {
		t.Fatal("edit")
	}
	del := `{"tool_name":"Delete","tool_input":{"path":"billing/a.py"}}`
	if Run(strings.NewReader(del), &strings.Builder{}, &strings.Builder{}, opts) != 0 && Run(strings.NewReader(del), &strings.Builder{}, &strings.Builder{}, opts) != 2 {
		t.Fatal("delete")
	}
	bare := `{"tool_name":"Write","tool_input":{"path":"billing/a.py"}}`
	_ = Run(strings.NewReader(bare), &strings.Builder{}, &strings.Builder{}, opts)
	shell := `{"tool_name":"Shell","command":"cat auth/a.py billing/a.py"}`
	if Run(strings.NewReader(shell), &strings.Builder{}, &strings.Builder{}, opts) != 2 {
		t.Fatal("denied shell")
	}
	if Run(strings.NewReader(`{"tool_name":"Shell","command":"echo ok"}`), &strings.Builder{}, &strings.Builder{}, opts) != 0 {
		t.Fatal("shell allow")
	}
	os.RemoveAll(filepath.Join(repo, ".proofsieve", "run"))
	os.WriteFile(filepath.Join(repo, ".proofsieve", "run"), []byte("x"), 0o644)
	if Run(strings.NewReader(payload), &strings.Builder{}, &strings.Builder{}, opts) != 2 {
		t.Fatal("session")
	}
	parsed, err := lockfile.Parse([]byte(lock))
	if err != nil {
		t.Fatal(err)
	}
	parsed.Blast.Denied = append(parsed.Blast.Denied, "/**", "*")
	if _, _, bad := shellBlocked("echo hello", parsed); bad {
		t.Fatal("star prefix")
	}
	if Run(strings.NewReader(`{"tool_name":"Shell","command":"cat other/a.py"}`), &strings.Builder{}, &strings.Builder{}, opts) != 2 {
		t.Fatal("outside")
	}
	if !isWrite("strreplace") || !isWrite("applypatch") || isWrite("shell") {
		t.Fatal("isWrite")
	}
	os.RemoveAll(filepath.Join(repo, ".proofsieve", "run"))
	os.MkdirAll(filepath.Join(repo, ".proofsieve", "run", "session.json"), 0o755)
	if Run(strings.NewReader(payload), &strings.Builder{}, &strings.Builder{}, opts) != 2 {
		t.Fatal("session load")
	}
	if looksPath("") || looksPath(".") || looksPath("..") || !looksPath("a.py") {
		t.Fatal("looks")
	}
	if str(nil, "command") != "" || first(map[string]any{"path": ""}, "path", "file_path") != "" {
		t.Fatal("str")
	}
}
