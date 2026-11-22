package check

import (
	"context"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/attest"
	"github.com/vaquarkhan/ProofSieve/internal/behavior"
)

func TestQuizFindings(t *testing.T) {
	diff := behavior.Diff{
		Branches: []string{"price"},
		Calls:    []behavior.Call{{Name: "eval"}},
	}
	if len(quizFindings(nil, diff)) != 2 {
		t.Fatal("missing answers")
	}
	wrong := &attest.Attestation{Branch: "other", Call: "nope"}
	if len(quizFindings(wrong, diff)) != 2 {
		t.Fatal("wrong answers")
	}
	right := &attest.Attestation{Branch: "price", Call: "eval"}
	if len(quizFindings(right, diff)) != 0 {
		t.Fatal("accepted")
	}
	if len(quizFindings(nil, behavior.Diff{})) != 0 {
		t.Fatal("nothing to ask")
	}
	onlyCall := behavior.Diff{Calls: []behavior.Call{{Name: "helper"}}}
	got := quizFindings(&attest.Attestation{Call: "helper"}, onlyCall)
	if len(got) != 0 {
		t.Fatal(got)
	}
}

func TestNoMutants(t *testing.T) {
	repo := initRepo(t)
	writeRel(t, repo, ".proofsieve/policy.yaml", pythonPolicy("")+`
mutation_gate:
  enabled: true
  test_command: ["test"]
`)
	writeRel(t, repo, "billing/price.py", "x = 1\n")
	writeRel(t, repo, ".proofsieve/scope.lock.yaml", lockYAML("feature", 6, 400, 15, `["**"]`, `[]`, ""))
	base := commitAll(t, repo, "base")
	writeRel(t, repo, "billing/price.py", "x = 2\n")
	commitAll(t, repo, "head")
	rep, code := Run(context.Background(), Options{Repo: repo, Base: base, Gate: "mutation"})
	if code != 0 || !hasRule(rep, "no-mutants") {
		t.Fatalf("%d %s", code, rep.JSON())
	}
}
