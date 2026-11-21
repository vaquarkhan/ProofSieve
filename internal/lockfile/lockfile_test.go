package lockfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndSave(t *testing.T) {
	repo := t.TempDir()
	oracle := filepath.Join(repo, "billing", "oracle")
	if err := os.MkdirAll(oracle, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("def test_ok():\n    assert True\n")
	if err := os.WriteFile(filepath.Join(oracle, "test_rounding.py"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := Generate(Options{
		Repo: repo, Spec: "specs/BUG-1.md", ChangeType: "bugfix", Owner: "dev",
		Allow: []string{"billing/**"}, LockGlobs: []string{"billing/oracle/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if lock.Blast.MaxFiles != 6 || lock.Blast.Denied[0] != ".proofsieve/**" || len(lock.Tests.Locked) != 1 {
		t.Fatalf("%+v", lock)
	}
	out := filepath.Join(repo, ".proofsieve", "scope.lock.yaml")
	if err := Save(out, lock); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tests.Locked[0].Path != "billing/oracle/test_rounding.py" || loaded.Hash == "" {
		t.Fatalf("%+v", loaded)
	}
	if !IsPolicyPath(loaded.Tests.Locked[0].Path) && !IsPolicyPath(".proofsieve/policy.yaml") {
		t.Fatal("policy path")
	}
	if IsPolicyPath("billing/rounding.py") {
		t.Fatal("not policy")
	}
}

func TestRejects(t *testing.T) {
	if _, err := Parse([]byte("version: \"1.0\"\n")); err == nil {
		t.Fatal("short")
	}
	if _, err := Generate(Options{Spec: "s", Allow: []string{"a/**"}, LockGlobs: []string{"missing/**"}, Repo: t.TempDir()}); err == nil {
		t.Fatal("no files")
	}
	if _, err := Parse([]byte("version: \"1.0\"\nspec_ref: s\nchange_type: bugfix\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: ['../x']\n  denied_paths: []\ntest_integrity:\n  allow_test_deletion: false\n  allow_skip_annotations: false\n  allow_test_mutation: false\n  locked_files: []\n")); err == nil {
		t.Fatal("dotdot")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("missing")
	}
	sum, err := Digest(filepath.Join(t.TempDir(), "nope"))
	if err == nil || sum != "" {
		t.Fatal("digest")
	}
}
