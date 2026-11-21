package mutate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateAndApplyEdges(t *testing.T) {
	if _, ok := (Result{}).Ratio(); ok {
		t.Fatal("ratio")
	}
	if Generate("a.py", "def f():\n    return 'nope\n", "python", nil) != nil {
		t.Fatal("lex")
	}
	if Generate("a_test.py", "def test():\n    assert True\n", "python", nil) != nil {
		t.Fatal("test file")
	}
	src := "flag = True\nconsole.log(1)\n"
	mutants := Generate("a.js", "function f(){ flag = true; console.log(1); return 1 }\n", "javascript", nil)
	mutants = append(mutants, Mutant{Path: "b.js", Line: 2, Tier: 2, Operator: "b", Start: 0, End: 1, Mutated: "x"})
	mutants = append(mutants, Mutant{Path: "a.js", Line: 9, Tier: 3, Operator: "z", Start: 0, End: 1, Mutated: "y"})
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "a.js"), []byte(src), 0o644)
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repo, "node_modules"), 0o755)
	os.MkdirAll(filepath.Join(repo, ".venv"), 0o755)
	os.MkdirAll(filepath.Join(repo, "venv"), 0o755)
	os.MkdirAll(filepath.Join(repo, "__pycache__"), 0o755)
	origTemp := mkdirTemp
	mkdirTemp = func(string, string) (string, error) { return "", errors.New("temp") }
	if _, err := Apply(context.Background(), repo, []Mutant{{Path: "a.js", Tier: 1, Start: 0, End: 1}}, []string{"test"}, time.Second, 0, func(context.Context, string, []string) (int, error) {
		return 0, nil
	}); err == nil {
		t.Fatal("temp")
	}
	mkdirTemp = origTemp
	if _, err := Apply(context.Background(), repo, nil, nil, time.Second, 1, nil); err == nil {
		t.Fatal("empty command")
	}
	if _, err := Apply(context.Background(), filepath.Join(repo, "missing"), mutants[:1], []string{"proofsieve-missing"}, time.Second, 0, nil); err == nil {
		t.Fatal("copy")
	}
	_, err := Apply(context.Background(), repo, []Mutant{{Path: "a.js", Tier: 2, Start: 0, End: 1, Mutated: "X"}}, []string{"proofsieve-missing"}, time.Second, 0, nil)
	if err == nil {
		t.Fatal("exec")
	}
	res, err := Apply(context.Background(), repo, []Mutant{
		{Path: "a.js", Line: 2, Tier: 2, Operator: "b", Start: 0, End: 1, Mutated: "X"},
		{Path: "a.js", Line: 1, Tier: 3, Operator: "a", Start: 0, End: 1, Mutated: "Y"},
		{Path: "z.js", Line: 1, Tier: 1, Operator: "c", Start: 0, End: 1, Mutated: "Z"},
	}, []string{"test"}, time.Second, 2, func(context.Context, string, []string) (int, error) {
		return 1, nil
	})
	if err != nil || res.Tier2Killed == 0 && res.Tier3Killed == 0 && res.Tier1Killed == 0 {
		t.Fatal(err, res)
	}
	if _, err := Apply(context.Background(), repo, []Mutant{{Path: "a.js", Tier: 1, Start: -1, End: 1}}, []string{"test"}, time.Second, 0, func(context.Context, string, []string) (int, error) {
		return 0, nil
	}); err == nil {
		t.Fatal("offset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	cancel()
	_, _ = DefaultExec(ctx, repo, []string{"proofsieve-missing"})
	if _, err := DefaultExec(context.Background(), repo, nil); err == nil {
		t.Fatal("nil command")
	}
	os.MkdirAll(filepath.Join(repo, "sub"), 0o755)
	os.WriteFile(filepath.Join(repo, "sub", "a.txt"), []byte("a"), 0o644)
	os.MkdirAll(filepath.Join(repo, "dir.js"), 0o755)
	if _, err := Apply(context.Background(), repo, []Mutant{{Path: "dir.js", Tier: 1, Start: 0, End: 0, Mutated: "x"}}, []string{"test"}, time.Second, 0, func(context.Context, string, []string) (int, error) {
		return 0, nil
	}); err == nil {
		t.Fatal("write dir")
	}
	origWrite, origRead, origRel, origMk := writeFile, readFile, relPath, mkdirAll
	t.Cleanup(func() {
		writeFile, readFile, relPath, mkdirAll = origWrite, origRead, origRel, origMk
	})
	writeFile = func(string, []byte, os.FileMode) error { return errors.New("write") }
	if _, err := Apply(context.Background(), repo, []Mutant{{Path: "a.js", Tier: 1, Start: 0, End: 1, Mutated: "Z"}}, []string{"test"}, time.Second, 0, func(context.Context, string, []string) (int, error) {
		return 1, nil
	}); err == nil {
		t.Fatal("write")
	}
	writeFile = origWrite
	relPath = func(string, string) (string, error) { return "", errors.New("rel") }
	if _, err := Apply(context.Background(), repo, nil, []string{"test"}, time.Second, 0, func(context.Context, string, []string) (int, error) {
		return 1, nil
	}); err == nil {
		t.Fatal("rel")
	}
	relPath = origRel
	readFile = func(string) ([]byte, error) { return nil, errors.New("read") }
	if _, err := Apply(context.Background(), repo, nil, []string{"test"}, time.Second, 0, func(context.Context, string, []string) (int, error) {
		return 1, nil
	}); err == nil {
		t.Fatal("read")
	}
	readFile = origRead
	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	if _, err := Apply(context.Background(), repo, nil, []string{"test"}, time.Second, 0, func(context.Context, string, []string) (int, error) {
		return 1, nil
	}); err == nil {
		t.Fatal("mkdir")
	}
	expired, stop := context.WithTimeout(context.Background(), time.Nanosecond)
	time.Sleep(20 * time.Millisecond)
	_, _ = DefaultExec(expired, repo, []string{"cmd", "/c", "exit", "0"})
	stop()
	_ = mutants
}
