// Package cli is the proofsieve command line.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vaquarkhan/ProofSieve/internal/attest"
	"github.com/vaquarkhan/ProofSieve/internal/check"
	"github.com/vaquarkhan/ProofSieve/internal/guard"
	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
	"github.com/vaquarkhan/ProofSieve/internal/report"
	"github.com/vaquarkhan/ProofSieve/internal/review"
	"github.com/vaquarkhan/ProofSieve/internal/scaffold"
	"github.com/vaquarkhan/ProofSieve/internal/testgen"
	"github.com/vaquarkhan/ProofSieve/internal/version"
)

var (
	marshalIndent = json.MarshalIndent
	readFile      = os.ReadFile
)

var reproduce = review.Reproduce

// Execute runs one invocation and returns the process exit code.
func Execute(args []string, in io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, version.String())
		return 0
	case "init":
		return cmdInit(args[1:], stdout, stderr)
	case "lock":
		return cmdLock(args[1:], stdout, stderr)
	case "guard":
		return guard.Run(in, stdout, stderr, guard.Options{})
	case "check":
		return cmdCheck(args[1:], stdout, stderr)
	case "mutate":
		return cmdMutate(args[1:], stdout, stderr)
	case "test-gen":
		return cmdTestGen(args[1:], stdout, stderr)
	case "verify-findings":
		return cmdVerify(args[1:], stdout, stderr)
	case "attest":
		return cmdAttest(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n%s", args[0], usage)
		return 2
	}
}

func cmdInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "repository root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	res, err := scaffold.Init(*repo)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprint(stdout, scaffold.Summary(res))
	return 0
}

func cmdLock(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lock", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "repository root")
	spec := fs.String("spec", "", "path to the spec")
	change := fs.String("change-type", "bugfix", "bugfix, feature, or refactor")
	owner := fs.String("owner", "", "accountable owner")
	maxFiles := fs.Int("max-files", 6, "file budget")
	maxLoc := fs.Int("max-loc", 250, "churn budget")
	churnPct := fs.Int("churn-pct", 15, "nonfunctional churn percent for bugfixes")
	out := fs.String("out", "", "lock path")
	var allow, deny, lockGlobs multi
	fs.Var(&allow, "allow", "allowed glob (repeatable)")
	fs.Var(&deny, "deny", "denied glob (repeatable)")
	fs.Var(&lockGlobs, "lock-glob", "oracle glob to hash (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *spec == "" || len(allow) == 0 {
		fmt.Fprintln(stderr, "lock requires --spec and at least one --allow")
		return 2
	}
	lock, err := lockfile.Generate(lockfile.Options{
		Repo: *repo, Spec: *spec, ChangeType: *change, Owner: *owner,
		MaxFiles: *maxFiles, MaxLoc: *maxLoc, ChurnPct: *churnPct,
		Allow: []string(allow), Deny: []string(deny), LockGlobs: []string(lockGlobs),
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	dest := *out
	if dest == "" {
		dest = filepath.Join(*repo, ".proofsieve", "scope.lock.yaml")
	}
	if err := lockfile.Save(dest, lock); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "wrote %s (%d locked tests)\n", dest, len(lock.Tests.Locked))
	return 0
}

func cmdCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "repository root")
	base := fs.String("base", "", "base revision")
	head := fs.String("head", "HEAD", "head revision")
	policyPath := fs.String("policy", "", "policy path")
	lockPath := fs.String("lock", "", "scope lock path")
	format := fs.String("format", "md", "json, md, or sarif")
	gate := fs.String("gate", "all", "all, scope, complexity, mutation, review, or ownership")
	attestFile := fs.String("attest-file", "", "attestation comment file")
	author := fs.String("attest-author", "", "human account that wrote the attestation")
	evidence := fs.String("evidence", "", "evidence JSON output path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *base == "" {
		fmt.Fprintln(stderr, "check requires --base")
		return 2
	}
	text := ""
	if *attestFile != "" {
		b, err := os.ReadFile(*attestFile)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		text = string(b)
	}
	rep, code := check.Run(context.Background(), check.Options{
		Repo: *repo, Base: *base, Head: *head, PolicyPath: *policyPath, LockPath: *lockPath,
		Gate: *gate, AttestText: text, AttestAuthor: *author,
	})
	if err := printReport(stdout, *format, rep); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *evidence != "" {
		if err := os.MkdirAll(filepath.Dir(*evidence), 0o755); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := os.WriteFile(*evidence, []byte(rep.JSON()), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	return code
}

func cmdMutate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mutate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "repository root")
	base := fs.String("base", "", "base revision")
	head := fs.String("head", "HEAD", "head revision")
	out := fs.String("out", "surviving_mutants.json", "survivor file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *base == "" {
		fmt.Fprintln(stderr, "mutate requires --base")
		return 2
	}
	rep, code := check.Run(context.Background(), check.Options{Repo: *repo, Base: *base, Head: *head, Gate: "mutation"})
	if err := printReport(stdout, "md", rep); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	raw, err := marshalIndent(map[string]any{"mutation_tier1": rep.MutationTier1}, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := os.WriteFile(*out, append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return code
}

func cmdTestGen(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("test-gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "repository root")
	spec := fs.String("spec", "", "spec path")
	out := fs.String("out", "", "output directory")
	var srcs multi
	fs.Var(&srcs, "src", "source file (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *spec == "" || *out == "" || len(srcs) == 0 {
		fmt.Fprintln(stderr, "test-gen requires --spec, --out, and --src")
		return 2
	}
	specText, err := os.ReadFile(filepath.Join(*repo, filepath.FromSlash(*spec)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	files := map[string]string{}
	for _, src := range srcs {
		b, err := os.ReadFile(filepath.Join(*repo, filepath.FromSlash(src)))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		files[src] = string(b)
	}
	bundle, err := testgen.Build(string(specText), files)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := testgen.Write(*out, bundle); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return 0
}

func cmdVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify-findings", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "repository root")
	base := fs.String("base", "", "base revision")
	head := fs.String("head", "HEAD", "head revision")
	input := fs.String("input", "", "raw findings JSON")
	out := fs.String("out", "", "kept findings JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *input == "" || *base == "" {
		fmt.Fprintln(stderr, "verify-findings requires --input and --base")
		return 2
	}
	b, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var claims []review.Claim
	if err := json.Unmarshal(b, &claims); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	proved := map[int]bool{}
	for i, c := range claims {
		if review.Unsafe(c.Test) {
			continue
		}
		basePass, headPass, err := reproduce(context.Background(), *repo, *base, *head, c.Test, []string{"proofsieve-test"}, nil)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		proved[i] = review.Match(c.Expect, basePass, headPass)
	}
	kept := review.Select(claims, map[string]bool{}, proved, 5)
	raw, err := marshalIndent(kept, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if *out != "" {
		if err := os.WriteFile(*out, append(raw, '\n'), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	fmt.Fprintln(stdout, string(raw))
	return 0
}

func cmdAttest(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "verify" {
		return cmdAttestVerify(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("attest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sha := fs.String("sha", "", "head commit")
	spec := fs.String("spec", "", "requirement id")
	invariant := fs.String("invariant", "", "invariant preserved")
	failure := fs.String("failure", "", "failure mode and rollback")
	testName := fs.String("test", "", "test name")
	branch := fs.String("branch", "", "quiz function that contains a conditional")
	call := fs.String("call", "", "quiz callee inside the diff")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	text := fmt.Sprintf("/proofsieve attest\nsha: %s\nspec: %s\ninvariant: %s\nfailure: %s\ntest: %s\n", *sha, *spec, *invariant, *failure, *testName)
	if *branch != "" {
		text += "branch: " + *branch + "\n"
	}
	if *call != "" {
		text += "call: " + *call + "\n"
	}
	if _, err := attest.Parse(text); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprint(stdout, text)
	return 0
}

func cmdAttestVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("attest verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "comment file")
	sha := fs.String("sha", "", "expected head sha")
	specFile := fs.String("spec-file", "", "spec file")
	repo := fs.String("repo", ".", "repository root")
	author := fs.String("author", "", "comment author")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	b, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !attest.AuthorOK(*author) {
		fmt.Fprintln(stderr, "author is not a human account")
		return 1
	}
	a, err := attest.Parse(string(b))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if a.SHA != *sha {
		fmt.Fprintln(stderr, "attestation sha does not match")
		return 1
	}
	id, err := attest.SpecID(a.Spec)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	spec, err := os.ReadFile(*specFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !attest.HoldsSpec(string(spec), id) || !holdsInRepo(*repo, a.Test) {
		fmt.Fprintln(stderr, "spec id or test name does not resolve")
		return 1
	}
	fmt.Fprintln(stdout, "attestation verified")
	return 0
}

func holdsInRepo(repo, name string) bool {
	found := false
	_ = filepath.WalkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found {
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

func printReport(w io.Writer, format string, rep report.Report) error {
	switch format {
	case "json":
		_, err := io.WriteString(w, rep.JSON())
		return err
	case "sarif":
		_, err := io.WriteString(w, rep.SARIF())
		return err
	case "md":
		_, err := io.WriteString(w, rep.Markdown())
		return err
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

type multi []string

func (m *multi) String() string { return strings.Join(*m, ",") }

func (m *multi) Set(v string) error {
	*m = append(*m, v)
	return nil
}

const usage = `proofsieve is the deterministic control plane for AI-written code.

Usage:
  proofsieve init [--repo .]
  proofsieve lock --spec <file> --allow <glob> [--lock-glob <glob>] [--repo .]
  proofsieve guard
  proofsieve check --base <rev> [--head HEAD] [--format md|json|sarif] [--gate all]
  proofsieve mutate --base <rev> [--head HEAD]
  proofsieve test-gen --spec <file> --src <file> --out <dir>
  proofsieve verify-findings --input <file> --base <rev>
  proofsieve attest --sha <sha> --spec <id> --invariant <text> --failure <text> --test <name> [--branch <fn> --call <callee>]
  proofsieve attest verify --file <comment> --sha <sha> --spec-file <file> --author <login>
  proofsieve version

Exit codes for check: 0 pass, 1 policy failure, 2 configuration or infrastructure error.
The guard exits 0 to allow a tool call and 2 to deny it.
`
