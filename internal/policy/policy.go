// Package policy loads .proofsieve/policy.yaml.
package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/vaquarkhan/ProofSieve/internal/behavior"
	"gopkg.in/yaml.v3"
)

// Policy is the repository control-plane policy.
type Policy struct {
	Version           string            `yaml:"version"`
	Languages         []string          `yaml:"languages"`
	ComplexityCaps    ComplexityCaps    `yaml:"complexity_caps"`
	MutationGate      MutationGate      `yaml:"mutation_gate"`
	ReviewTriage      ReviewTriage      `yaml:"review_triage"`
	ComprehensionGate ComprehensionGate `yaml:"comprehension_gate"`
	Hash              string            `yaml:"-"`
}

// ComplexityCaps limits functions the diff adds or edits.
type ComplexityCaps struct {
	MaxCognitive int    `yaml:"max_cognitive_per_function"`
	MaxNesting   int    `yaml:"max_nesting_depth"`
	MaxLines     int    `yaml:"max_function_lines"`
	ApplyTo      string `yaml:"apply_to"`
}

// MutationGate configures diff-scoped mutation.
type MutationGate struct {
	Enabled         bool              `yaml:"enabled"`
	IncrementalOnly *bool             `yaml:"incremental_only"`
	MinTier1        int               `yaml:"min_tier1_kill_rate_pct"`
	MinTier2        int               `yaml:"min_tier2_kill_rate_pct"`
	Tier3Gates      *bool             `yaml:"tier3_gates"`
	Adapters        map[string]string `yaml:"adapters"`
	TestCommand     []string          `yaml:"test_command"`
	MaxMutants      int               `yaml:"max_mutants"`
	TimeoutSec      int               `yaml:"timeout_seconds"`
}

// ReviewTriage configures deterministic review publication.
type ReviewTriage struct {
	Enabled             bool       `yaml:"enabled"`
	SARIF               []string   `yaml:"tier0_sarif_paths"`
	RequireReproduction bool       `yaml:"require_reproduction"`
	MaxComments         int        `yaml:"max_inline_comments"`
	IgnoreRegistry      string     `yaml:"ignore_registry"`
	AllowRemote         *bool      `yaml:"allow_remote_model"`
	TestCommand         []string   `yaml:"test_command"`
	Sandbox             string     `yaml:"sandbox"`
	DockerImage         string     `yaml:"docker_image"`
	Tier0Commands       [][]string `yaml:"tier0_commands"`
}

// ComprehensionGate configures the human ownership check.
type ComprehensionGate struct {
	Enabled            bool     `yaml:"enabled"`
	RiskThreshold      int      `yaml:"risk_threshold"`
	Weights            *Weights `yaml:"weights"`
	CriticalPaths      []string `yaml:"critical_paths"`
	RequireAttestation bool     `yaml:"require_structured_attestation"`
	RequireQuiz        bool     `yaml:"require_diff_quiz"`
	SensitiveCalls     []string `yaml:"sensitive_calls"`
}

// Weights are the fixed risk-score coefficients.
type Weights struct {
	Files     float64 `yaml:"files"`
	Cognitive float64 `yaml:"cognitive"`
	Critical  float64 `yaml:"critical_path"`
	Mutation  float64 `yaml:"mutation_gap"`
}

// Load reads a policy file from disk.
func Load(path string) (Policy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, err
	}
	return Parse(b)
}

// Parse decodes and validates policy YAML.
func Parse(b []byte) (Policy, error) {
	var p Policy
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return Policy{}, fmt.Errorf("policy: %w", err)
	}
	sum := sha256.Sum256(b)
	p.Hash = hex.EncodeToString(sum[:])
	if err := p.normalize(); err != nil {
		return Policy{}, err
	}
	return p, nil
}

func (p *Policy) normalize() error {
	if p.Version != "1.0" {
		return fmt.Errorf("policy version %q is not supported", p.Version)
	}
	if len(p.Languages) == 0 {
		p.Languages = []string{"python", "typescript"}
	}
	for _, lang := range p.Languages {
		switch lang {
		case "typescript", "javascript", "python", "go", "rust", "java", "kotlin":
		default:
			return fmt.Errorf("policy language %q is unknown", lang)
		}
	}
	if p.ComplexityCaps.MaxCognitive <= 0 || p.ComplexityCaps.MaxNesting <= 0 || p.ComplexityCaps.MaxLines <= 0 {
		return fmt.Errorf("complexity caps must be positive")
	}
	if p.ComplexityCaps.ApplyTo == "" {
		p.ComplexityCaps.ApplyTo = "added_or_modified_functions"
	}
	if p.ComplexityCaps.ApplyTo != "added_or_modified_functions" {
		return fmt.Errorf("complexity apply_to must be added_or_modified_functions")
	}
	if p.MutationGate.IncrementalOnly != nil && !*p.MutationGate.IncrementalOnly {
		return fmt.Errorf("mutation_gate.incremental_only must be true")
	}
	if p.MutationGate.Tier3Gates != nil && *p.MutationGate.Tier3Gates {
		return fmt.Errorf("mutation_gate.tier3_gates must be false")
	}
	if p.MutationGate.MinTier1 == 0 {
		p.MutationGate.MinTier1 = 80
	}
	if p.MutationGate.MinTier2 == 0 {
		p.MutationGate.MinTier2 = 70
	}
	if p.MutationGate.MinTier1 < 0 || p.MutationGate.MinTier1 > 100 || p.MutationGate.MinTier2 < 0 || p.MutationGate.MinTier2 > 100 {
		return fmt.Errorf("mutation kill rates must be between 1 and 100")
	}
	if p.MutationGate.MaxMutants == 0 {
		p.MutationGate.MaxMutants = 200
	}
	if p.MutationGate.MaxMutants < 1 {
		return fmt.Errorf("max_mutants must be positive")
	}
	if p.MutationGate.TimeoutSec == 0 {
		p.MutationGate.TimeoutSec = 30
	}
	if p.ReviewTriage.AllowRemote != nil && *p.ReviewTriage.AllowRemote {
		return fmt.Errorf("allow_remote_model must be false")
	}
	if p.ReviewTriage.MaxComments == 0 {
		p.ReviewTriage.MaxComments = 5
	}
	if p.ReviewTriage.MaxComments < 0 || p.ReviewTriage.MaxComments > 5 {
		return fmt.Errorf("max_inline_comments must be from 1 to 5")
	}
	if p.ReviewTriage.IgnoreRegistry == "" {
		p.ReviewTriage.IgnoreRegistry = ".proofsieve/ignore.json"
	}
	if p.ReviewTriage.Sandbox == "" {
		p.ReviewTriage.Sandbox = "worktree"
	}
	if p.ReviewTriage.Sandbox != "worktree" && p.ReviewTriage.Sandbox != "docker" {
		return fmt.Errorf("sandbox must be worktree or docker")
	}
	if p.ComprehensionGate.RiskThreshold == 0 {
		p.ComprehensionGate.RiskThreshold = 60
	}
	if p.ComprehensionGate.RiskThreshold < 0 || p.ComprehensionGate.RiskThreshold > 100 {
		return fmt.Errorf("risk_threshold must be from 1 to 100")
	}
	if len(p.ComprehensionGate.SensitiveCalls) == 0 {
		p.ComprehensionGate.SensitiveCalls = behavior.DefaultSensitive()
	}
	if p.ComprehensionGate.Weights == nil {
		p.ComprehensionGate.Weights = &Weights{Files: 0.25, Cognitive: 0.20, Critical: 0.35, Mutation: 0.20}
	} else if p.ComprehensionGate.Weights.Files != 0.25 || p.ComprehensionGate.Weights.Cognitive != 0.20 || p.ComprehensionGate.Weights.Critical != 0.35 || p.ComprehensionGate.Weights.Mutation != 0.20 {
		return fmt.Errorf("risk weights are fixed at 0.25, 0.20, 0.35, 0.20")
	}
	return nil
}

// AllowsLanguage reports whether a source language may appear in a diff.
func (p Policy) AllowsLanguage(lang string) bool {
	for _, item := range p.Languages {
		if item == lang {
			return true
		}
	}
	return false
}
