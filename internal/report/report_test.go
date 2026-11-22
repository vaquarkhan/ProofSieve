package report

import (
	"strings"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/behavior"
)

func TestFormats(t *testing.T) {
	ratio := 0.5
	risk := 74
	r := Report{
		Version: "1.0.0", SHA: "abc", Files: 2, DiffLOC: 10,
		MutationTier1: &ratio, RiskScore: &risk, FindingsRaw: 3, FindingsKept: 1,
		Gates:    []Gate{{Name: "scope", Status: "fail"}},
		Findings: []Finding{{Gate: "scope", Severity: "error", Rule: "path", Path: "a.py", Line: 4, Message: "outside scope"}},
	}
	if !r.Failed() {
		t.Fatal("failed")
	}
	if !strings.Contains(r.JSON(), "\"rule\": \"path\"") {
		t.Fatal(r.JSON())
	}
	if !strings.Contains(r.Markdown(), "Risk score: 74") || !strings.Contains(r.Markdown(), "a.py:4") {
		t.Fatal(r.Markdown())
	}
	if !strings.Contains(r.SARIF(), "\"ruleId\": \"path\"") {
		t.Fatal(r.SARIF())
	}
	empty := Report{Version: "1.0.0"}
	if empty.Failed() || !strings.Contains(empty.Markdown(), "No findings") {
		t.Fatal(empty.Markdown())
	}
	warn := Report{Findings: []Finding{{Severity: "warning", Rule: "t2", Message: "low", Path: "b.py"}}}
	if warn.Failed() || !strings.Contains(warn.SARIF(), "warning") {
		t.Fatal(warn.SARIF())
	}
	note := Report{Findings: []Finding{{Severity: "info", Rule: "n", Message: "m"}}}
	if !strings.Contains(note.SARIF(), "note") {
		t.Fatal(note.SARIF())
	}
	with := Report{Version: "1.0.0", Behavior: &behavior.Diff{
		SpecRef: "specs/a.md",
		Changes: []behavior.Change{{Kind: "added", Name: "price", Path: "a.py", Line: 1}},
		Calls:   []behavior.Call{{Name: "eval", Path: "a.py", Line: 2, Sensitive: true}, {Name: "helper", Path: "a.py", Line: 3}},
		Callers: []behavior.Caller{{Name: "use", Path: "b.py", Line: 4, Callee: "price"}},
	}}
	md := with.Markdown()
	if !strings.Contains(md, "added price") || !strings.Contains(md, "sensitive eval") || !strings.Contains(md, "caller use") || !strings.Contains(with.JSON(), "specs/a.md") {
		t.Fatal(md)
	}
	none := Report{Behavior: &behavior.Diff{SpecRef: "specs/a.md"}}
	if !strings.Contains(none.Markdown(), "no signature changes") {
		t.Fatal(none.Markdown())
	}
}
