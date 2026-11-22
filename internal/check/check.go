// Package check runs the ProofSieve gates against a git range.
package check

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vaquarkhan/ProofSieve/internal/attest"
	"github.com/vaquarkhan/ProofSieve/internal/behavior"
	"github.com/vaquarkhan/ProofSieve/internal/churn"
	"github.com/vaquarkhan/ProofSieve/internal/diffparse"
	"github.com/vaquarkhan/ProofSieve/internal/gitx"
	"github.com/vaquarkhan/ProofSieve/internal/globset"
	"github.com/vaquarkhan/ProofSieve/internal/integrity"
	"github.com/vaquarkhan/ProofSieve/internal/lang"
	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
	"github.com/vaquarkhan/ProofSieve/internal/mutate"
	"github.com/vaquarkhan/ProofSieve/internal/policy"
	"github.com/vaquarkhan/ProofSieve/internal/report"
	"github.com/vaquarkhan/ProofSieve/internal/review"
	"github.com/vaquarkhan/ProofSieve/internal/risk"
	"github.com/vaquarkhan/ProofSieve/internal/scope"
	"github.com/vaquarkhan/ProofSieve/internal/version"
)

var (
	getwd    = os.Getwd
	diffFn   = gitx.Diff
	showFn   = gitx.Show
	readFile = os.ReadFile
)

// Options is one check invocation.
type Options struct {
	Repo         string
	Base         string
	Head         string
	PolicyPath   string
	LockPath     string
	Gate         string
	AttestText   string
	AttestAuthor string
	MutateExec   mutate.Exec
	ReviewExec   review.Exec
}

// Run executes the requested gates. The integer is the process exit code.
func Run(ctx context.Context, opts Options) (report.Report, int) {
	rep := report.Report{Version: version.Version, Findings: []report.Finding{}}
	if opts.Gate == "" {
		opts.Gate = "all"
	}
	if opts.Head == "" {
		opts.Head = "HEAD"
	}
	if opts.Repo == "" {
		wd, err := getwd()
		if err != nil {
			return infra(rep, err.Error()), 2
		}
		opts.Repo = wd
	}
	if opts.PolicyPath == "" {
		opts.PolicyPath = filepath.Join(opts.Repo, ".proofsieve", "policy.yaml")
	}
	if opts.LockPath == "" {
		opts.LockPath = filepath.Join(opts.Repo, ".proofsieve", "scope.lock.yaml")
	}
	pol, err := policy.Load(opts.PolicyPath)
	if err != nil {
		return infra(rep, "policy: "+err.Error()), 2
	}
	lock, err := lockfile.Load(opts.LockPath)
	if err != nil {
		return infra(rep, "scope lock: "+err.Error()), 2
	}
	rep.PolicyHash = pol.Hash
	rep.LockHash = lock.Hash
	sha, err := gitx.Rev(ctx, opts.Repo, opts.Head)
	if err != nil {
		return infra(rep, err.Error()), 2
	}
	rep.SHA = sha
	diffText, err := diffFn(ctx, opts.Repo, opts.Base, opts.Head)
	if err != nil {
		return infra(rep, err.Error()), 2
	}
	files, err := diffparse.Parse(diffText)
	if err != nil {
		return infra(rep, err.Error()), 2
	}
	rep.Files = len(files)
	for _, f := range files {
		rep.DiffLOC += f.Added + f.Removed
	}
	bdiff := behavior.Build(ctx, opts.Repo, opts.Base, lock.SpecRef, files, pol.ComprehensionGate.SensitiveCalls)
	rep.Behavior = &bdiff
	attested := attestationOK(opts, lock, sha)
	var parsed *attest.Attestation
	if a, err := attest.Parse(opts.AttestText); err == nil {
		parsed = &a
	}
	_, baseLockOK, err := showFn(ctx, opts.Repo, opts.Base, ".proofsieve/scope.lock.yaml")
	if err != nil {
		return infra(rep, err.Error()), 2
	}
	if want(opts.Gate, "scope") {
		before := len(rep.Findings)
		rep.Findings = append(rep.Findings, scopeFindings(ctx, opts, lock, files, attested, baseLockOK)...)
		rep.Findings = append(rep.Findings, lockFindings(opts, lock)...)
		if lock.ChangeType == "bugfix" {
			rep.Findings = append(rep.Findings, churnFindings(files, lock)...)
		}
		rep.Gates = append(rep.Gates, statusGate("scope", rep.Findings[before:]))
	}
	if want(opts.Gate, "complexity") {
		before := len(rep.Findings)
		rep.Findings = append(rep.Findings, complexityFindings(opts, pol, files)...)
		rep.Gates = append(rep.Gates, statusGate("complexity", rep.Findings[before:]))
	}
	cog := cognitiveAdded(opts, files)
	var ratio *float64
	if pol.MutationGate.Enabled && want(opts.Gate, "mutation") {
		before := len(rep.Findings)
		findings, r, code, err := mutationFindings(ctx, opts, pol, files)
		if err != nil {
			return infra(rep, err.Error()), code
		}
		ratio = r
		rep.MutationTier1 = r
		rep.Findings = append(rep.Findings, findings...)
		rep.Gates = append(rep.Gates, statusGate("mutation", rep.Findings[before:]))
	}
	if pol.ReviewTriage.Enabled && want(opts.Gate, "review") {
		before := len(rep.Findings)
		findings, raw, kept, code, err := reviewFindings(ctx, opts, pol)
		if err != nil {
			return infra(rep, err.Error()), code
		}
		rep.FindingsRaw = raw
		rep.FindingsKept = kept
		rep.Findings = append(rep.Findings, findings...)
		rep.Gates = append(rep.Gates, statusGate("review", rep.Findings[before:]))
	}
	if pol.ComprehensionGate.Enabled && want(opts.Gate, "ownership") {
		before := len(rep.Findings)
		score := risk.Score(risk.Input{
			Files: len(files), MaxFiles: lock.Blast.MaxFiles, CognitiveAdded: cog,
			Critical: critical(pol, files), MutationEnabled: pol.MutationGate.Enabled,
			MutationKnown: ratio != nil, KillRatio: deref(ratio),
		})
		rep.RiskScore = &score
		rep.Findings = append(rep.Findings, ownershipFindings(pol, score, attested, parsed, bdiff)...)
		rep.Gates = append(rep.Gates, statusGate("ownership", rep.Findings[before:]))
	}
	if rep.Failed() {
		return rep, 1
	}
	return rep, 0
}

func want(gate, name string) bool {
	return gate == "all" || gate == name
}

func infra(rep report.Report, msg string) report.Report {
	rep.Findings = append(rep.Findings, report.Finding{Gate: "infra", Severity: "error", Rule: "infra", Message: msg})
	return rep
}

func statusGate(name string, findings []report.Finding) report.Gate {
	status := "pass"
	for _, f := range findings {
		if f.Severity == "error" && (f.Gate == name || f.Gate == "infra") {
			status = "fail"
		}
	}
	return report.Gate{Name: name, Status: status}
}

func scopeFindings(ctx context.Context, opts Options, lock lockfile.Lock, files []diffparse.File, attested, baseLock bool) []report.Finding {
	var out []report.Finding
	loc := 0
	for _, f := range files {
		loc += f.Added + f.Removed
	}
	if len(files) > lock.Blast.MaxFiles {
		out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "max-files", Message: "diff touches more files than the scope lock allows"})
	}
	if loc > lock.Blast.MaxLoc {
		out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "max-loc", Message: "diff churn exceeds the scope lock"})
	}
	for _, f := range files {
		path := f.Path()
		if lockfile.IsPolicyPath(path) {
			if !baseLock || attested {
				continue
			}
			out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "policy-path", Path: path, Message: "editing .proofsieve requires a human attestation"})
			continue
		}
		d := scope.Decide(lock, path, 0, 0, false)
		if !d.Allow {
			out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: d.Rule, Path: path, Message: d.Message})
		}
		if !lang.IsTest(path) {
			continue
		}
		base, _, err := showFn(ctx, opts.Repo, opts.Base, path)
		if err != nil {
			out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "infra", Path: path, Message: err.Error()})
			continue
		}
		head := ""
		if !f.Deleted() {
			b, err := os.ReadFile(filepath.Join(opts.Repo, filepath.FromSlash(path)))
			if err != nil {
				out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "infra", Path: path, Message: err.Error()})
				continue
			}
			head = string(b)
		}
		out = append(out, integrity.Check(path, string(base), head, lock)...)
	}
	return out
}

func lockFindings(opts Options, lock lockfile.Lock) []report.Finding {
	var out []report.Finding
	for _, locked := range lock.Tests.Locked {
		full := filepath.Join(opts.Repo, filepath.FromSlash(locked.Path))
		b, err := os.ReadFile(full)
		if err != nil {
			out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "locked-test", Path: locked.Path, Message: "locked test is missing"})
			continue
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != locked.SHA256 {
			out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "locked-test", Path: locked.Path, Message: "locked test digest changed"})
		}
	}
	return out
}

func complexityFindings(opts Options, pol policy.Policy, files []diffparse.File) []report.Finding {
	var out []report.Finding
	for _, f := range files {
		if f.Deleted() {
			continue
		}
		language, ok := lang.Language(f.Path())
		if !ok {
			continue
		}
		if !pol.AllowsLanguage(language) {
			out = append(out, report.Finding{Gate: "complexity", Severity: "error", Rule: "language", Path: f.Path(), Message: language + " is not allowed by policy"})
			continue
		}
		if !lang.Supported(language) {
			out = append(out, report.Finding{Gate: "complexity", Severity: "error", Rule: "unparsed", Path: f.Path(), Message: "no analyzer for " + language})
			continue
		}
		b, err := os.ReadFile(filepath.Join(opts.Repo, filepath.FromSlash(f.Path())))
		if err != nil {
			out = append(out, report.Finding{Gate: "complexity", Severity: "error", Rule: "infra", Path: f.Path(), Message: err.Error()})
			continue
		}
		fns, err := lang.Analyze(language, string(b))
		if err != nil {
			out = append(out, report.Finding{Gate: "complexity", Severity: "error", Rule: "unparsed", Path: f.Path(), Message: err.Error()})
			continue
		}
		touched := diffparse.NewLines(f)
		if f.AddedFile() {
			touched = allLines(string(b))
		}
		for _, fn := range fns {
			if !overlaps(fn, touched) {
				continue
			}
			if fn.Cognitive > pol.ComplexityCaps.MaxCognitive || fn.MaxNesting > pol.ComplexityCaps.MaxNesting || fn.Lines > pol.ComplexityCaps.MaxLines {
				out = append(out, report.Finding{Gate: "complexity", Severity: "error", Rule: "complexity", Path: f.Path(), Line: fn.StartLine, Message: fn.Name + " exceeds the complexity cap"})
			}
		}
	}
	return out
}

func allLines(src string) map[int]bool {
	out := map[int]bool{}
	for i := range strings.Split(src, "\n") {
		out[i+1] = true
	}
	return out
}

func overlaps(fn lang.Function, lines map[int]bool) bool {
	for line := range lines {
		if line >= fn.StartLine && line <= fn.EndLine {
			return true
		}
	}
	return false
}

func churnFindings(files []diffparse.File, lock lockfile.Lock) []report.Finding {
	non, total := 0, 0
	for _, f := range files {
		language, ok := lang.Language(f.Path())
		if !ok || !lang.Supported(language) {
			continue
		}
		for _, h := range f.Hunks {
			total += churn.Lines(h.OldText, h.NewText)
			class := churn.Class(language, h.OldText, h.NewText)
			if class == "formatting" || class == "rename" {
				non += churn.Lines(h.OldText, h.NewText)
			}
		}
	}
	if total == 0 {
		return nil
	}
	pct := non * 100 / total
	if pct > lock.Blast.ChurnPct {
		return []report.Finding{{Gate: "scope", Severity: "error", Rule: "churn", Message: "bugfix diff is mostly formatting or rename"}}
	}
	return nil
}

func cognitiveAdded(opts Options, files []diffparse.File) int {
	sum := 0
	for _, f := range files {
		language, ok := lang.Language(f.Path())
		if !ok || !lang.Supported(language) || f.Deleted() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(opts.Repo, filepath.FromSlash(f.Path())))
		if err != nil {
			continue
		}
		fns, err := lang.Analyze(language, string(b))
		if err != nil {
			continue
		}
		for _, fn := range fns {
			if overlaps(fn, diffparse.NewLines(f)) || f.AddedFile() {
				sum += fn.Cognitive
			}
		}
	}
	return sum
}

func critical(pol policy.Policy, files []diffparse.File) bool {
	set, err := globset.Compile(pol.ComprehensionGate.CriticalPaths)
	if err != nil {
		return false
	}
	for _, f := range files {
		if set.Match(f.Path()) {
			return true
		}
	}
	return false
}

func mutationFindings(ctx context.Context, opts Options, pol policy.Policy, files []diffparse.File) ([]report.Finding, *float64, int, error) {
	if len(pol.MutationGate.TestCommand) == 0 {
		return nil, nil, 2, errString("mutation test_command is empty")
	}
	var mutants []mutate.Mutant
	for _, f := range files {
		if f.Deleted() || lang.IsTest(f.Path()) {
			continue
		}
		language, ok := lang.Language(f.Path())
		if !ok || !lang.Supported(language) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(opts.Repo, filepath.FromSlash(f.Path())))
		if err != nil {
			return nil, nil, 2, err
		}
		mutants = append(mutants, mutate.Generate(f.Path(), string(b), language, diffparse.NewLines(f))...)
	}
	if len(mutants) == 0 {
		return []report.Finding{{Gate: "mutation", Severity: "warning", Rule: "no-mutants", Message: "no tier-1 mutants on the diff"}}, nil, 0, nil
	}
	res, err := mutate.Apply(ctx, opts.Repo, mutants, pol.MutationGate.TestCommand, time.Duration(pol.MutationGate.TimeoutSec)*time.Second, pol.MutationGate.MaxMutants, opts.MutateExec)
	if err != nil {
		return nil, nil, 2, err
	}
	ratio, ok := res.Ratio()
	var findings []report.Finding
	var ratioPtr *float64
	if ok {
		ratioPtr = &ratio
	}
	if res.Truncated {
		findings = append(findings, report.Finding{Gate: "mutation", Severity: "warning", Rule: "mutants-truncated", Message: "mutant list was truncated at the configured cap"})
	}
	if res.Tier1Total > 0 && res.Tier1Killed*100 < pol.MutationGate.MinTier1*res.Tier1Total {
		findings = append(findings, report.Finding{Gate: "mutation", Severity: "error", Rule: "mutation-score", Message: "tier-1 mutation kill rate is below the policy threshold"})
	}
	if res.Tier2Total > 0 && res.Tier2Killed*100 < pol.MutationGate.MinTier2*res.Tier2Total {
		findings = append(findings, report.Finding{Gate: "mutation", Severity: "warning", Rule: "mutation-tier2", Message: "tier-2 mutation kill rate is below the advisory threshold"})
	}
	return findings, ratioPtr, 0, nil
}

func reviewFindings(ctx context.Context, opts Options, pol policy.Policy) ([]report.Finding, int, int, int, error) {
	var claims []review.Claim
	proved := map[int]bool{}
	for _, rel := range pol.ReviewTriage.SARIF {
		b, err := os.ReadFile(filepath.Join(opts.Repo, filepath.FromSlash(rel)))
		if err != nil {
			return nil, 0, 0, 2, err
		}
		parsed, err := review.ParseSARIF(b)
		if err != nil {
			return nil, 0, 0, 2, err
		}
		for _, c := range parsed {
			proved[len(claims)] = true
			c.Test = "sarif"
			claims = append(claims, c)
		}
	}
	rawPath := filepath.Join(opts.Repo, ".proofsieve", "review.json")
	if b, err := os.ReadFile(rawPath); err == nil {
		var extra []review.Claim
		if err := json.Unmarshal(b, &extra); err != nil {
			return nil, 0, 0, 2, err
		}
		for _, c := range extra {
			idx := len(claims)
			claims = append(claims, c)
			if review.Unsafe(c.Test) {
				continue
			}
			if len(pol.ReviewTriage.TestCommand) == 0 {
				return nil, len(claims), 0, 2, errString("review test_command is empty")
			}
			basePass, headPass, err := review.Reproduce(ctx, opts.Repo, opts.Base, opts.Head, c.Test, pol.ReviewTriage.TestCommand, opts.ReviewExec)
			if err != nil {
				return nil, 0, 0, 2, err
			}
			proved[idx] = review.Match(c.Expect, basePass, headPass)
		}
	} else if !os.IsNotExist(err) {
		return nil, 0, 0, 2, err
	}
	ignore, err := review.ReadIgnoreFile(filepath.Join(opts.Repo, filepath.FromSlash(pol.ReviewTriage.IgnoreRegistry)))
	if err != nil {
		return nil, 0, 0, 2, err
	}
	kept := review.Select(claims, ignore, proved, pol.ReviewTriage.MaxComments)
	var findings []report.Finding
	for _, c := range kept {
		if c.Test == "sarif" || c.Severity == "error" {
			findings = append(findings, report.Finding{Gate: "review", Severity: c.Severity, Rule: c.Rule, Path: c.Path, Line: c.Line, Message: c.Claim})
		}
	}
	return findings, len(claims), len(kept), 0, nil
}

func ownershipFindings(pol policy.Policy, score int, attested bool, att *attest.Attestation, diff behavior.Diff) []report.Finding {
	var out []report.Finding
	if pol.ComprehensionGate.RequireQuiz {
		out = append(out, quizFindings(att, diff)...)
	}
	if score >= pol.ComprehensionGate.RiskThreshold && pol.ComprehensionGate.RequireAttestation && !attested {
		out = append(out, report.Finding{Gate: "ownership", Severity: "error", Rule: "attestation", Message: "high-risk diff needs a human /proofsieve attest comment"})
	}
	return out
}

func quizFindings(att *attest.Attestation, diff behavior.Diff) []report.Finding {
	var out []report.Finding
	branch, call := "", ""
	if att != nil {
		branch, call = att.Branch, att.Call
	}
	if len(diff.Branches) > 0 && !diff.HasBranch(branch) {
		out = append(out, report.Finding{Gate: "ownership", Severity: "error", Rule: "quiz", Message: "diff quiz requires branch: set to a changed function that contains a conditional"})
	}
	if len(diff.Calls) > 0 && !diff.HasCall(call) {
		out = append(out, report.Finding{Gate: "ownership", Severity: "error", Rule: "quiz", Message: "diff quiz requires call: set to a callee inside a changed function"})
	}
	return out
}

func attestationOK(opts Options, lock lockfile.Lock, sha string) bool {
	if !attest.AuthorOK(opts.AttestAuthor) || strings.TrimSpace(opts.AttestText) == "" {
		return false
	}
	a, err := attest.Parse(opts.AttestText)
	if err != nil || a.SHA != sha {
		return false
	}
	id, err := attest.SpecID(a.Spec)
	if err != nil {
		return false
	}
	spec, err := os.ReadFile(filepath.Join(opts.Repo, filepath.FromSlash(lock.SpecRef)))
	if err != nil || !attest.HoldsSpec(string(spec), id) {
		return false
	}
	return testExists(opts.Repo, a.Test)
}

func testExists(repo, name string) bool {
	found := false
	_ = filepath.WalkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == ".git" || d.Name() == ".proofsieve") {
				return filepath.SkipDir
			}
			return nil
		}
		if found {
			return nil
		}
		b, err := readFile(path)
		if err != nil {
			return nil
		}
		if attest.HoldsTest(string(b), name) {
			found = true
		}
		return nil
	})
	return found
}

func deref(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

type errString string

func (e errString) Error() string { return string(e) }
