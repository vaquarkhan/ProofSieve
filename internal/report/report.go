// Package report renders gate results as JSON, Markdown, and SARIF.
package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vaquarkhan/ProofSieve/internal/behavior"
)

var marshalIndent = json.MarshalIndent

// Finding is one gate result row.
type Finding struct {
	Gate     string `json:"gate"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
}

// Gate is the pass, fail, or skip status of one gate.
type Gate struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Report is the evidence document for one commit.
type Report struct {
	Version       string         `json:"version"`
	SHA           string         `json:"sha,omitempty"`
	PolicyHash    string         `json:"policy_hash,omitempty"`
	LockHash      string         `json:"lock_hash,omitempty"`
	Gates         []Gate         `json:"gates"`
	Findings      []Finding      `json:"findings"`
	DiffLOC       int            `json:"diff_loc"`
	Files         int            `json:"files"`
	MutationTier1 *float64       `json:"mutation_tier1,omitempty"`
	FindingsRaw   int            `json:"findings_raw"`
	FindingsKept  int            `json:"findings_kept"`
	RiskScore     *int           `json:"risk_score,omitempty"`
	Behavior      *behavior.Diff `json:"behavior,omitempty"`
}

// Failed reports whether any finding is an error.
func (r Report) Failed() bool {
	for _, f := range r.Findings {
		if f.Severity == "error" {
			return true
		}
	}
	return false
}

// JSON returns indented JSON.
func (r Report) JSON() string {
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	if r.Gates == nil {
		r.Gates = []Gate{}
	}
	b, err := marshalIndent(r, "", "  ")
	if err != nil {
		return "{\"error\":\"marshal\"}\n"
	}
	return string(b) + "\n"
}

// Markdown returns a pull-request summary.
func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "ProofSieve %s\n\n", r.Version)
	fmt.Fprintf(&b, "Files: %d. Churn lines: %d.\n\n", r.Files, r.DiffLOC)
	if r.Behavior != nil {
		fmt.Fprintf(&b, "Spec: %s\n", r.Behavior.SpecRef)
		if len(r.Behavior.Changes) == 0 {
			b.WriteString("Behavioral diff: no signature changes.\n")
		}
		for _, c := range r.Behavior.Changes {
			fmt.Fprintf(&b, "- %s %s %s:%d\n", c.Kind, c.Name, c.Path, c.Line)
		}
		for _, c := range r.Behavior.Calls {
			if c.Sensitive {
				fmt.Fprintf(&b, "- sensitive %s %s:%d\n", c.Name, c.Path, c.Line)
			}
		}
		for _, c := range r.Behavior.Callers {
			fmt.Fprintf(&b, "- caller %s %s:%d -> %s\n", c.Name, c.Path, c.Line, c.Callee)
		}
		b.WriteString("\n")
	}
	if r.RiskScore != nil {
		fmt.Fprintf(&b, "Risk score: %d.\n\n", *r.RiskScore)
	}
	if r.MutationTier1 != nil {
		fmt.Fprintf(&b, "Tier-1 kill ratio: %.4f.\n\n", *r.MutationTier1)
	}
	for _, g := range r.Gates {
		fmt.Fprintf(&b, "- %s: %s\n", g.Name, g.Status)
	}
	if len(r.Findings) == 0 {
		b.WriteString("\nNo findings.\n")
		return b.String()
	}
	b.WriteString("\n")
	for _, f := range r.Findings {
		loc := f.Path
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.Path, f.Line)
		}
		fmt.Fprintf(&b, "- [%s] %s %s %s\n", f.Severity, f.Rule, loc, f.Message)
	}
	return b.String()
}

// SARIF returns a SARIF 2.1.0 document.
func (r Report) SARIF() string {
	type msg struct {
		Text string `json:"text"`
	}
	type loc struct {
		Physical struct {
			Artifact struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
			Region struct {
				StartLine int `json:"startLine"`
			} `json:"region"`
		} `json:"physicalLocation"`
	}
	type result struct {
		RuleID  string `json:"ruleId"`
		Level   string `json:"level"`
		Message msg    `json:"message"`
		Locs    []loc  `json:"locations,omitempty"`
	}
	results := make([]result, 0, len(r.Findings))
	for _, f := range r.Findings {
		level := "note"
		if f.Severity == "error" {
			level = "error"
		} else if f.Severity == "warning" {
			level = "warning"
		}
		item := result{RuleID: f.Rule, Level: level, Message: msg{Text: f.Message}}
		if f.Path != "" {
			var place loc
			place.Physical.Artifact.URI = f.Path
			place.Physical.Region.StartLine = f.Line
			if place.Physical.Region.StartLine == 0 {
				place.Physical.Region.StartLine = 1
			}
			item.Locs = []loc{place}
		}
		results = append(results, item)
	}
	doc := map[string]any{
		"version": "2.1.0",
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"runs": []any{map[string]any{
			"tool":    map[string]any{"driver": map[string]any{"name": "proofsieve", "version": r.Version}},
			"results": results,
		}},
	}
	b, err := marshalIndent(doc, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(b) + "\n"
}
