// Package scaffold writes the ProofSieve project files.
package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	marshalIndent = json.MarshalIndent
	writeFile     = os.WriteFile
	statPath      = os.Stat
)

const policyYAML = `version: "1.0"
languages:
  - python
  - typescript
complexity_caps:
  max_cognitive_per_function: 10
  max_nesting_depth: 4
  max_function_lines: 50
  apply_to: added_or_modified_functions
mutation_gate:
  enabled: false
  incremental_only: true
  min_tier1_kill_rate_pct: 80
  min_tier2_kill_rate_pct: 70
  tier3_gates: false
  test_command: []
  max_mutants: 200
  timeout_seconds: 30
review_triage:
  enabled: false
  tier0_sarif_paths: []
  require_reproduction: true
  max_inline_comments: 5
  ignore_registry: .proofsieve/ignore.json
  allow_remote_model: false
  sandbox: worktree
comprehension_gate:
  enabled: false
  risk_threshold: 60
  critical_paths:
    - "**/auth/**"
    - "**/payments/**"
    - "**/migrations/**"
  require_structured_attestation: true
  require_diff_quiz: false
`

// Result describes files that were created or left in place.
type Result struct {
	Created []string
	Kept    []string
}

// Init writes policy, hook config, and a gitignore entry when they are absent.
func Init(repo string) (Result, error) {
	var res Result
	policy := filepath.Join(repo, ".proofsieve", "policy.yaml")
	if err := writeNew(policy, policyYAML, &res); err != nil {
		return res, err
	}
	hooks := map[string]any{
		"version": 1,
		"hooks": map[string]any{
			"preToolUse": []any{
				map[string]string{"matcher": "Write|Delete|Shell", "command": "proofsieve guard"},
			},
			"beforeShellExecution": []any{
				map[string]string{"command": "proofsieve guard"},
			},
		},
	}
	raw, err := marshalIndent(hooks, "", "  ")
	if err != nil {
		return res, err
	}
	if err := writeNew(filepath.Join(repo, ".cursor", "hooks.json"), string(raw)+"\n", &res); err != nil {
		return res, err
	}
	claude := "{\n  \"hooks\": {\n    \"PreToolUse\": [\n      {\n        \"matcher\": \"Edit|Write|Bash\",\n        \"hooks\": [\n          {\"type\": \"command\", \"command\": \"proofsieve guard\"}\n        ]\n      }\n    ]\n  }\n}\n"
	if err := writeNew(filepath.Join(repo, ".claude", "settings.json"), claude, &res); err != nil {
		return res, err
	}
	if err := ensureIgnore(repo, &res); err != nil {
		return res, err
	}
	return res, nil
}

func writeNew(path, body string, res *Result) error {
	if _, err := statPath(path); err == nil {
		res.Kept = append(res.Kept, path)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := writeFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	res.Created = append(res.Created, path)
	return nil
}

func ensureIgnore(repo string, res *Result) error {
	path := filepath.Join(repo, ".gitignore")
	line := ".proofsieve/run/"
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(b), line) {
		res.Kept = append(res.Kept, path)
		return nil
	}
	if len(b) > 0 && !strings.HasSuffix(string(b), "\n") {
		b = append(b, '\n')
	}
	b = append(b, []byte(line+"\n")...)
	if err := writeFile(path, b, 0o644); err != nil {
		return err
	}
	res.Created = append(res.Created, path)
	return nil
}

// Summary renders init for a human.
func Summary(res Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Created %d file(s), kept %d.\n", len(res.Created), len(res.Kept))
	b.WriteString("Next: proofsieve lock --spec <spec.md> --allow <glob> --lock-glob <oracle-glob>\n")
	return b.String()
}
