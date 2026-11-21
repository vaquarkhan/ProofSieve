package lockfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndGenerateEdges(t *testing.T) {
	cases := []string{
		"version: \"9\"\n",
		"version: \"1.0\"\nchange_type: other\nspec_ref: a\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: [\"**\"]\n",
		"version: \"1.0\"\nchange_type: bugfix\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: [\"**\"]\n",
		"version: \"1.0\"\nchange_type: bugfix\nspec_ref: a\nblast_radius:\n  max_files: 0\n  max_loc_delta: 1\n  allowed_paths: [\"**\"]\n",
		"version: \"1.0\"\nchange_type: bugfix\nspec_ref: a\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  max_nonfunctional_churn_pct: 101\n  allowed_paths: [\"**\"]\n",
		"version: \"1.0\"\nchange_type: bugfix\nspec_ref: a\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: []\n",
		"version: \"1.0\"\nchange_type: bugfix\nspec_ref: a\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: [\"..\"]\n",
		"version: \"1.0\"\nchange_type: bugfix\nspec_ref: a\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: [\"**\"]\n  denied_paths: [\"..\"]\n",
		"version: \"1.0\"\nchange_type: bugfix\nspec_ref: a\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: [\"**\"]\ntest_integrity:\n  locked_files:\n    - path: \"\"\n      sha256: " + zero + "\n",
		"version: \"1.0\"\nchange_type: bugfix\nspec_ref: a\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: [\"**\"]\ntest_integrity:\n  locked_files:\n    - path: a.py\n      sha256: " + upper + "\n",
		"nope: true\n",
	}
	for _, body := range cases {
		if _, err := Parse([]byte(body)); err == nil {
			t.Fatal(body)
		}
	}
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.WriteFile(filepath.Join(repo, ".git", "x"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(repo, "node_modules"), 0o755)
	os.MkdirAll(filepath.Join(repo, ".venv"), 0o755)
	os.WriteFile(filepath.Join(repo, "a.py"), []byte("def test():\n    pass\n"), 0o644)
	if _, err := Generate(Options{Repo: repo, Spec: "s", Allow: []string{"**"}, Deny: []string{".proofsieve/**"}, LockGlobs: []string{"**"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(Options{Repo: repo, Allow: []string{"**"}, LockGlobs: []string{"missing/**"}}); err == nil {
		t.Fatal("no match")
	}
	if _, err := Generate(Options{Repo: repo, Allow: []string{"**"}, LockGlobs: []string{".."}}); err == nil {
		t.Fatal("glob")
	}
	if _, err := Generate(Options{Repo: t.TempDir(), Allow: nil}); err == nil {
		t.Fatal("normalize")
	}
	if _, err := Digest(filepath.Join(repo, "missing.py")); err == nil {
		t.Fatal("digest")
	}
	if _, err := Load(filepath.Join(repo, "missing.lock")); err == nil {
		t.Fatal("load")
	}
	file := filepath.Join(repo, "notdir")
	os.WriteFile(file, []byte("x"), 0o644)
	lock, err := Parse([]byte("version: \"1.0\"\nchange_type: feature\nspec_ref: a\nblast_radius:\n  max_files: 1\n  max_loc_delta: 1\n  allowed_paths: [\"**\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(t.TempDir(), "bad.yaml"), Lock{}); err == nil {
		t.Fatal("normalize")
	}
	if err := Save(filepath.Join(file, "lock.yaml"), lock); err == nil {
		t.Fatal("save")
	}
	ro := readFile
	readFile = func(string) ([]byte, error) { return nil, errors.New("read") }
	if _, err := Generate(Options{Repo: repo, Spec: "s", Allow: []string{"**"}, LockGlobs: []string{"**"}}); err == nil {
		t.Fatal("read")
	}
	readFile = ro
	orig := yamlMarshal
	yamlMarshal = func(any) ([]byte, error) { return nil, errors.New("yaml") }
	t.Cleanup(func() { yamlMarshal = orig })
	if err := Save(filepath.Join(t.TempDir(), "lock.yaml"), lock); err == nil {
		t.Fatal("marshal")
	}
	if len(PolicyPaths()) != 3 || !IsPolicyPath(".proofsieve/policy.yaml") || IsPolicyPath("a.py") {
		t.Fatal("policy paths")
	}
}

func TestWalkFailures(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "a.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := walkDir
	t.Cleanup(func() { walkDir = orig })
	walkDir = func(_ string, fn fs.WalkDirFunc) error {
		return fn(repo, nil, errors.New("walk"))
	}
	if _, err := hashGlobs(repo, []string{"**"}); err == nil {
		t.Fatal("walk")
	}
	entries, err := os.ReadDir(repo)
	if err != nil || len(entries) == 0 {
		t.Fatal(err)
	}
	walkDir = func(_ string, fn fs.WalkDirFunc) error {
		return fn(`D:\outside\a.py`, entries[0], nil)
	}
	if _, err := hashGlobs(repo, []string{"**"}); err == nil {
		t.Fatal("rel")
	}
}

const zero = "0000000000000000000000000000000000000000000000000000000000000000"
const upper = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
