package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	p, err := Parse([]byte("version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.MutationGate.MinTier1 != 80 || p.ReviewTriage.MaxComments != 5 || p.ComprehensionGate.RiskThreshold != 60 {
		t.Fatalf("%+v", p)
	}
	if !p.AllowsLanguage("python") || p.AllowsLanguage("rust") {
		t.Fatal("languages")
	}
	if p.Hash == "" {
		t.Fatal("hash")
	}
	found := false
	for _, name := range p.ComprehensionGate.SensitiveCalls {
		if name == "eval" {
			found = true
		}
	}
	if !found {
		t.Fatal(p.ComprehensionGate.SensitiveCalls)
	}
	custom, err := Parse([]byte("version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\ncomprehension_gate:\n  sensitive_calls: [system]\n"))
	if err != nil || len(custom.ComprehensionGate.SensitiveCalls) != 1 || custom.ComprehensionGate.SensitiveCalls[0] != "system" {
		t.Fatal(err, custom.ComprehensionGate.SensitiveCalls)
	}
}

func TestParseRejects(t *testing.T) {
	cases := []string{
		"version: \"9\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 0\n  max_nesting_depth: 4\n  max_function_lines: 50\n",
		"version: \"1.0\"\nlanguages: [cobol]\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\n  apply_to: everything\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nmutation_gate:\n  incremental_only: false\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nmutation_gate:\n  tier3_gates: true\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nmutation_gate:\n  min_tier1_kill_rate_pct: 101\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nreview_triage:\n  allow_remote_model: true\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nreview_triage:\n  max_inline_comments: 9\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nreview_triage:\n  sandbox: vm\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\ncomprehension_gate:\n  weights:\n    files: 0.5\n    cognitive: 0.2\n    critical_path: 0.35\n    mutation_gap: 0.2\n",
		"version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\nnope: true\n",
	}
	for i, c := range cases {
		if _, err := Parse([]byte(c)); err == nil {
			t.Fatalf("case %d accepted\n%s", i, c)
		}
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(t.TempDir() + "/missing.yaml"); err == nil {
		t.Fatal("missing")
	}
}

func TestExamplePolicy(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "examples", "policy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(b); err != nil {
		t.Fatal(err)
	}
}

func TestRiskBounds(t *testing.T) {
	src := "version: \"1.0\"\ncomplexity_caps:\n  max_cognitive_per_function: 10\n  max_nesting_depth: 4\n  max_function_lines: 50\ncomprehension_gate:\n  risk_threshold: 140\n"
	if _, err := Parse([]byte(src)); err == nil {
		t.Fatal("threshold")
	}
}
