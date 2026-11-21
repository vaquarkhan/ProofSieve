// Package review keeps a finding only when a named test proves it.
package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/vaquarkhan/ProofSieve/internal/gitx"
)

var mkdirTemp = os.MkdirTemp

// Expectation is the required base and head result.
type Expectation struct {
	Base string `json:"base"`
	Head string `json:"head"`
}

// Claim is one proposed finding.
type Claim struct {
	Claim    string      `json:"claim"`
	Test     string      `json:"test"`
	Expect   Expectation `json:"expect"`
	Severity string      `json:"severity"`
	Rule     string      `json:"rule"`
	Path     string      `json:"path"`
	Line     int         `json:"line"`
}

// Unsafe reports whether a test name could be a shell command.
func Unsafe(name string) bool {
	if strings.TrimSpace(name) == "" {
		return true
	}
	return strings.ContainsAny(name, "\n|&;`$<>")
}

// Match reports whether the observed results equal the expectation.
func Match(expect Expectation, basePass, headPass bool) bool {
	if expect.Base == "" || expect.Head == "" {
		return false
	}
	return pass(expect.Base) == basePass && pass(expect.Head) == headPass
}

func pass(v string) bool { return v == "pass" }

// Fingerprint identifies a dismissed finding.
func Fingerprint(c Claim) string {
	return strings.ToLower(strings.Join([]string{c.Rule, c.Path, fmt.Sprint(c.Line), strings.Join(strings.Fields(c.Claim), " ")}, "|"))
}

// Select keeps proved claims up to max, skipping ignored fingerprints.
func Select(claims []Claim, ignore map[string]bool, proved map[int]bool, max int) []Claim {
	var kept []Claim
	for i, c := range claims {
		if Unsafe(c.Test) || !proved[i] {
			continue
		}
		if ignore[Fingerprint(c)] {
			continue
		}
		if c.Severity == "" {
			c.Severity = "error"
		}
		kept = append(kept, c)
	}
	sort.SliceStable(kept, func(i, j int) bool {
		return rank(kept[i].Severity) < rank(kept[j].Severity)
	})
	if max >= 0 && len(kept) > max {
		kept = kept[:max]
	}
	return kept
}

func rank(s string) int {
	switch s {
	case "error":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

// ParseSARIF reads a SARIF 2.1.0 document into claims. SARIF claims are already proved.
func ParseSARIF(b []byte) ([]Claim, error) {
	var doc struct {
		Runs []struct {
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []struct {
					Physical struct {
						Artifact struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("sarif: %w", err)
	}
	var out []Claim
	for _, run := range doc.Runs {
		for _, res := range run.Results {
			c := Claim{Rule: res.RuleID, Claim: res.Message.Text, Severity: sarifLevel(res.Level), Test: "sarif"}
			if len(res.Locations) > 0 {
				c.Path = res.Locations[0].Physical.Artifact.URI
				c.Line = res.Locations[0].Physical.Region.StartLine
			}
			out = append(out, c)
		}
	}
	return out, nil
}

func sarifLevel(level string) string {
	switch level {
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

// LoadIgnore reads a fingerprint list.
func LoadIgnore(b []byte) (map[string]bool, error) {
	if len(bytesTrim(b)) == 0 {
		return map[string]bool{}, nil
	}
	var doc struct {
		Fingerprints []string `json:"fingerprints"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, f := range doc.Fingerprints {
		out[f] = true
	}
	return out, nil
}

func bytesTrim(b []byte) []byte { return []byte(strings.TrimSpace(string(b))) }

// Exec runs a command in a directory.
type Exec func(ctx context.Context, dir string, command []string) (int, error)

// Reproduce runs testName in base and head worktrees.
func Reproduce(ctx context.Context, repo, base, head, testName string, command []string, run Exec) (bool, bool, error) {
	if Unsafe(testName) {
		return false, false, fmt.Errorf("test name is not a single test id")
	}
	if run == nil {
		return false, false, fmt.Errorf("no command runner")
	}
	baseDir, err := mkdirTemp("", "proofsieve-base-*")
	if err != nil {
		return false, false, err
	}
	headDir, err := mkdirTemp("", "proofsieve-head-*")
	if err != nil {
		os.RemoveAll(baseDir)
		return false, false, err
	}
	defer os.RemoveAll(baseDir)
	defer os.RemoveAll(headDir)
	if err := gitx.AddWorktree(ctx, repo, baseDir, base); err != nil {
		return false, false, err
	}
	defer gitx.RemoveWorktree(ctx, repo, baseDir)
	if err := gitx.AddWorktree(ctx, repo, headDir, head); err != nil {
		return false, false, err
	}
	defer gitx.RemoveWorktree(ctx, repo, headDir)
	basePass, err := passed(ctx, run, baseDir, append(append([]string{}, command...), testName))
	if err != nil {
		return false, false, err
	}
	headPass, err := passed(ctx, run, headDir, append(append([]string{}, command...), testName))
	if err != nil {
		return false, false, err
	}
	return basePass, headPass, nil
}

func passed(ctx context.Context, run Exec, dir string, command []string) (bool, error) {
	code, err := run(ctx, dir, command)
	if err != nil {
		return false, err
	}
	return code == 0, nil
}

// DockerArgs is the rootless container invocation for a reproduction.
func DockerArgs(image, dir string, command []string) ([]string, error) {
	if image == "" {
		return nil, fmt.Errorf("docker image is empty")
	}
	args := []string{"run", "--rm", "--network=none", "--read-only", "--cpus=1", "--memory=512m", "-v", dir + ":/work:ro", "-w", "/work", image}
	return append(args, command...), nil
}

// ReadIgnoreFile loads an ignore registry when it exists.
func ReadIgnoreFile(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	return LoadIgnore(b)
}
