package mutate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	src := "def price(x):\n    if x > 1:\n        return x\n    console.log('no')\n    return None\n"
	got := Generate("billing/price.py", src, "python", map[int]bool{2: true, 3: true, 4: true})
	if len(got) < 2 {
		t.Fatalf("%+v", got)
	}
	if Generate("billing/test_price.py", src, "python", nil) != nil {
		t.Fatal("test file")
	}
	if Generate("a.py", src, "rust", nil) != nil {
		t.Fatal("rust")
	}
	if Generate("a.py", "def f(:\n", "python", nil) != nil {
		t.Fatal("bad source")
	}
	js := "function f(x) {\n  if (x === 1) { return x; }\n  let ok = true;\n  i++;\n}\n"
	if len(Generate("a.ts", js, "typescript", nil)) == 0 {
		t.Fatal("ts")
	}
}

func TestApplyKillAndSurvive(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "sample.py"), []byte("value = 1 > 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mutants := Generate("sample.py", "value = 1 > 0\n", "python", nil)
	if len(mutants) == 0 {
		t.Fatal("no mutants")
	}
	run := func(ctx context.Context, dir string, command []string) (int, error) {
		b, err := os.ReadFile(filepath.Join(dir, "sample.py"))
		if err != nil {
			return 0, err
		}
		if string(b) != "value = 1 > 0\n" {
			return 1, nil
		}
		return 0, nil
	}
	res, err := Apply(context.Background(), repo, mutants, []string{"test"}, time.Second, 10, run)
	if err != nil {
		t.Fatal(err)
	}
	ratio, ok := res.Ratio()
	if !ok || ratio != 1 {
		t.Fatalf("%+v %v", res, ratio)
	}
	survive := func(ctx context.Context, dir string, command []string) (int, error) { return 0, nil }
	res, err = Apply(context.Background(), repo, mutants[:1], []string{"test"}, time.Second, 0, survive)
	if err != nil || len(res.Survivors) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, ok := res.Ratio(); !ok && res.Tier1Total == 0 {
		t.Fatal("ratio")
	}
	if _, err := Apply(context.Background(), repo, mutants, nil, time.Second, 1, run); err == nil {
		t.Fatal("empty command")
	}
	many := append(append([]Mutant{}, mutants...), mutants...)
	res, err = Apply(context.Background(), repo, many, []string{"test"}, time.Second, 1, run)
	if err != nil || !res.Truncated {
		t.Fatal(res, err)
	}
}

func TestDefaultExec(t *testing.T) {
	if os.Getenv("PROOFSIEVE_HELPER") == "1" {
		if os.Getenv("PROOFSIEVE_HELPER_FAIL") == "1" {
			os.Exit(1)
		}
		os.Exit(0)
	}
	t.Setenv("PROOFSIEVE_HELPER", "1")
	code, err := DefaultExec(context.Background(), t.TempDir(), []string{os.Args[0], "-test.run=^TestDefaultExec$"})
	if err != nil || code != 0 {
		t.Fatalf("code %d err %v", code, err)
	}
	t.Setenv("PROOFSIEVE_HELPER_FAIL", "1")
	code, err = DefaultExec(context.Background(), t.TempDir(), []string{os.Args[0], "-test.run=^TestDefaultExec$"})
	if err != nil || code == 0 {
		t.Fatalf("fail path %d %v", code, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DefaultExec(ctx, t.TempDir(), []string{os.Args[0], "-test.run=^TestDefaultExec$"}); err == nil {
		t.Fatal("wanted cancelled command")
	}
	if _, err := DefaultExec(context.Background(), t.TempDir(), []string{"proofsieve-missing-bin"}); err == nil {
		t.Fatal("missing bin")
	}
	if _, err := DefaultExec(context.Background(), t.TempDir(), nil); err == nil {
		t.Fatal("empty")
	}
}

func TestApplyErrors(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "sample.py"), []byte("value = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := []Mutant{{Path: "missing.py", Tier: 1, Start: 0, End: 1, Mutated: "x"}}
	run := func(context.Context, string, []string) (int, error) { return 0, nil }
	if _, err := Apply(context.Background(), repo, bad, []string{"t"}, time.Second, 0, run); err == nil {
		t.Fatal("missing file")
	}
	bad[0].Path = "sample.py"
	bad[0].End = 50
	if _, err := Apply(context.Background(), repo, bad, []string{"t"}, time.Second, 0, run); err == nil {
		t.Fatal("offset")
	}
	boom := func(context.Context, string, []string) (int, error) { return 0, os.ErrClosed }
	ok := []Mutant{{Path: "sample.py", Tier: 1, Start: 0, End: 1, Original: "v", Mutated: "X"}}
	if _, err := Apply(context.Background(), repo, ok, []string{"t"}, time.Second, 0, boom); err == nil {
		t.Fatal("run")
	}
	dead := func(context.Context, string, []string) (int, error) { return 1, context.DeadlineExceeded }
	res, err := Apply(context.Background(), repo, ok, []string{"t"}, time.Second, 0, dead)
	if err != nil || res.Tier1Killed != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := Apply(context.Background(), filepath.Join(repo, "sample.py"), ok, []string{"t"}, time.Second, 0, run); err == nil {
		t.Fatal("copy")
	}
}
