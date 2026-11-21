package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/review"
)

func TestFlagAndIOEdges(t *testing.T) {
	var out, errb bytes.Buffer
	for _, args := range [][]string{
		{"lock", "-bad"},
		{"check", "-bad"},
		{"mutate", "-bad"},
		{"test-gen", "-bad"},
		{"verify-findings", "-bad"},
		{"attest", "-bad"},
		{"attest", "verify", "-bad"},
	} {
		if Execute(args, strings.NewReader(""), &out, &errb) != 2 {
			t.Fatal(args)
		}
	}
	repo := t.TempDir()
	file := filepath.Join(repo, "file")
	os.WriteFile(file, []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(repo, "billing"), 0o755)
	os.WriteFile(filepath.Join(repo, "billing", "a.py"), []byte("x = 1\n"), 0o644)
	if Execute([]string{"lock", "--repo", repo, "--spec", "s", "--allow", "billing/**", "--out", filepath.Join(file, "lock.yaml")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("save")
	}
	orig := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("json") }
	t.Cleanup(func() { marshalIndent = orig })
	if Execute([]string{"mutate", "--repo", repo, "--base", "HEAD", "--out", filepath.Join(repo, "s.json")}, strings.NewReader(""), &out, &errb) != 2 {
		t.Fatal("mutate marshal")
	}
	marshalIndent = orig
	findings := filepath.Join(repo, "findings.json")
	os.WriteFile(findings, []byte(`[{"claim":"c","test":"test_a","expect":{"base":"pass","head":"fail"}}]`), 0o644)
	badOut := &failWriter{}
	_ = Execute([]string{"check", "--repo", repo, "--base", "HEAD", "--evidence", filepath.Join(file, "e.json")}, strings.NewReader(""), &out, &errb)
	_ = Execute([]string{"check", "--repo", repo, "--base", "HEAD", "--evidence", repo}, strings.NewReader(""), &out, &errb)
	_ = Execute([]string{"mutate", "--repo", repo, "--base", "HEAD"}, strings.NewReader(""), badOut, &errb)
	_ = Execute([]string{"mutate", "--repo", repo, "--base", "HEAD", "--out", filepath.Join(file, "s.json")}, strings.NewReader(""), &out, &errb)
	os.WriteFile(filepath.Join(repo, "a.py"), []byte("# return value\ndef f():\n    return value\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "spec.md"), []byte("spec"), 0o644)
	_ = Execute([]string{"test-gen", "--repo", repo, "--spec", "spec.md", "--src", "a.py", "--out", filepath.Join(repo, "gen")}, strings.NewReader(""), &out, &errb)
	os.WriteFile(filepath.Join(repo, "b.go"), []byte("package p\nfunc F() int {\n    return 1\n}\n"), 0o644)
	_ = Execute([]string{"test-gen", "--repo", repo, "--spec", "spec.md", "--src", "b.go", "--out", file}, strings.NewReader(""), &out, &errb)
	os.WriteFile(findings, []byte(`[{"claim":"c","test":"a|b","expect":{"base":"pass","head":"fail"}}]`), 0o644)
	_ = Execute([]string{"verify-findings", "--base", "HEAD", "--input", findings}, strings.NewReader(""), &out, &errb)
	comment := filepath.Join(repo, "comment.txt")
	os.WriteFile(comment, []byte("/proofsieve attest\nsha: abc\nspec: specs/a.md\ninvariant: rounds half up\nfailure: revert the pricing change\ntest: test_a\n"), 0o644)
	_ = Execute([]string{"check", "--repo", repo, "--base", "HEAD", "--attest-file", comment}, strings.NewReader(""), &out, &errb)
	os.WriteFile(filepath.Join(repo, "spec.md"), []byte("other"), 0o644)
	_ = Execute([]string{"attest", "verify", "--file", comment, "--sha", "abc", "--spec-file", filepath.Join(repo, "spec.md"), "--author", "dev"}, strings.NewReader(""), &out, &errb)
	os.WriteFile(filepath.Join(repo, "spec.md"), []byte("REQ-1\n"), 0o644)
	os.WriteFile(comment, []byte("/proofsieve attest\nsha: abc\nspec: REQ-1\ninvariant: rounds half up\nfailure: revert the pricing change\ntest: test_a\n"), 0o644)
	empty := t.TempDir()
	_ = Execute([]string{"attest", "verify", "--file", comment, "--sha", "abc", "--spec-file", filepath.Join(repo, "spec.md"), "--repo", empty, "--author", "dev"}, strings.NewReader(""), &out, &errb)
	prev := reproduce
	reproduce = func(context.Context, string, string, string, string, []string, review.Exec) (bool, bool, error) {
		return true, false, nil
	}
	t.Cleanup(func() { reproduce = prev })
	_ = Execute([]string{"verify-findings", "--base", "HEAD", "--input", findings, "--out", filepath.Join(file, "out.json")}, strings.NewReader(""), &out, &errb)
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("json") }
	os.WriteFile(findings, []byte(`[{"claim":"c","test":"test_a","expect":{"base":"pass","head":"fail"}}]`), 0o644)
	_ = Execute([]string{"verify-findings", "--base", "HEAD", "--input", findings}, strings.NewReader(""), &out, &errb)
}

func TestHoldsReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := readFile
	readFile = func(string) ([]byte, error) { return nil, errors.New("read") }
	t.Cleanup(func() { readFile = orig })
	if holdsInRepo(dir, "name") {
		t.Fatal("readable")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write") }
