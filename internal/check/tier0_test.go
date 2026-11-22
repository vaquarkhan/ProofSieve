package check

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/policy"
)

func TestTier0Command(t *testing.T) {
	if os.Getenv("PROOFSIEVE_TIER0") == "1" {
		if os.Getenv("PROOFSIEVE_TIER0_OK") == "1" {
			os.Exit(0)
		}
		os.Exit(1)
	}
	orig := runTier0
	t.Cleanup(func() { runTier0 = orig })
	sarif := []byte(`{"runs":[{"results":[{"ruleId":"r","level":"error","message":{"text":"boom"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.py"},"region":{"startLine":1}}}]}]}]}`)
	repo := t.TempDir()
	pol := policy.Policy{ReviewTriage: policy.ReviewTriage{Tier0Commands: [][]string{{"semgrep"}}, MaxComments: 5, IgnoreRegistry: ".proofsieve/ignore.json"}}
	runTier0 = func(context.Context, string, []string) ([]byte, int, error) { return sarif, 0, nil }
	findings, raw, kept, code, err := reviewFindings(context.Background(), Options{Repo: repo}, pol)
	if err != nil || code != 0 || raw != 1 || kept != 1 || len(findings) != 1 {
		t.Fatalf("%v %d %d %d %+v", err, code, raw, kept, findings)
	}
	runTier0 = func(context.Context, string, []string) ([]byte, int, error) { return nil, 0, nil }
	if _, _, _, _, err = reviewFindings(context.Background(), Options{Repo: repo}, pol); err != nil {
		t.Fatal(err)
	}
	runTier0 = func(context.Context, string, []string) ([]byte, int, error) { return []byte("{"), 0, nil }
	if _, _, _, code, err = reviewFindings(context.Background(), Options{Repo: repo}, pol); err == nil || code != 2 {
		t.Fatal(err, code)
	}
	runTier0 = func(context.Context, string, []string) ([]byte, int, error) { return nil, 1, nil }
	if _, _, _, code, err = reviewFindings(context.Background(), Options{Repo: repo}, pol); err == nil || code != 2 {
		t.Fatal("status", err, code)
	}
	runTier0 = func(context.Context, string, []string) ([]byte, int, error) { return nil, 2, errors.New("run") }
	if _, _, _, code, err = reviewFindings(context.Background(), Options{Repo: repo}, pol); err == nil || code != 2 {
		t.Fatal("exec", err, code)
	}

	if _, code, err = defaultTier0(context.Background(), repo, nil); err == nil || code != 2 {
		t.Fatal("empty", err, code)
	}
	if _, _, err = defaultTier0(context.Background(), repo, []string{"proofsieve-missing-tier0"}); err == nil {
		t.Fatal("missing")
	}
	t.Setenv("PROOFSIEVE_TIER0", "1")
	t.Setenv("PROOFSIEVE_TIER0_OK", "1")
	if _, code, err = defaultTier0(context.Background(), repo, []string{os.Args[0], "-test.run=^TestTier0Command$"}); err != nil || code != 0 {
		t.Fatalf("ok child %d %v", code, err)
	}
	t.Setenv("PROOFSIEVE_TIER0_OK", "")
	if _, code, err = defaultTier0(context.Background(), repo, []string{os.Args[0], "-test.run=^TestTier0Command$"}); err != nil || code == 0 {
		t.Fatalf("fail child %d %v", code, err)
	}
}
