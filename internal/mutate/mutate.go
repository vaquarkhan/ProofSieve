// Package mutate generates diff-scoped mutants and runs the project's tests against each one.
package mutate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vaquarkhan/ProofSieve/internal/lang"
)

var (
	mkdirTemp = os.MkdirTemp
	writeFile = os.WriteFile
	relPath   = filepath.Rel
	readFile  = os.ReadFile
	mkdirAll  = os.MkdirAll
)

// Mutant is one source edit on a changed line.
type Mutant struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Tier     int    `json:"tier"`
	Operator string `json:"operator"`
	Original string `json:"original"`
	Mutated  string `json:"mutated"`
	Start    int    `json:"-"`
	End      int    `json:"-"`
}

// Result is the kill summary.
type Result struct {
	Tier1Killed int      `json:"tier1_killed"`
	Tier1Total  int      `json:"tier1_total"`
	Tier2Killed int      `json:"tier2_killed"`
	Tier2Total  int      `json:"tier2_total"`
	Tier3Killed int      `json:"tier3_killed"`
	Tier3Total  int      `json:"tier3_total"`
	Truncated   bool     `json:"truncated"`
	Survivors   []Mutant `json:"survivors"`
}

// Ratio is the tier-1 kill ratio. ok is false when there are no tier-1 mutants.
func (r Result) Ratio() (float64, bool) {
	if r.Tier1Total == 0 {
		return 0, false
	}
	return float64(r.Tier1Killed) / float64(r.Tier1Total), true
}

// Generate returns mutants for lines. A nil line set means every line.
func Generate(path, source, language string, lines map[int]bool) []Mutant {
	if lang.IsTest(path) || !lang.Supported(language) {
		return nil
	}
	toks, err := lang.Tokens(language, source)
	if err != nil {
		return nil
	}
	var out []Mutant
	for _, tok := range toks {
		if lines != nil && !lines[tok.Line] {
			continue
		}
		if m, ok := opMutant(path, tok); ok {
			out = append(out, m)
		}
	}
	out = append(out, returnMutants(path, source, language, lines)...)
	out = append(out, logMutants(path, source, lines)...)
	return out
}

func opMutant(path string, tok lang.Token) (Mutant, bool) {
	repl := map[string]struct {
		to, op string
		tier   int
	}{
		">":   {">=", "conditional_boundary", 1},
		"<":   {"<=", "conditional_boundary", 1},
		">=":  {">", "conditional_boundary", 1},
		"<=":  {"<", "conditional_boundary", 1},
		"==":  {"!=", "negate_conditional", 1},
		"!=":  {"==", "negate_conditional", 1},
		"===": {"!==", "negate_conditional", 1},
		"!==": {"===", "negate_conditional", 1},
		"+":   {"-", "math_operator", 1},
		"-":   {"+", "math_operator", 1},
		"*":   {"/", "math_operator", 1},
		"/":   {"*", "math_operator", 1},
		"++":  {"--", "incdec", 2},
		"--":  {"++", "incdec", 2},
	}
	if item, ok := repl[tok.Text]; ok && tok.Kind == "op" {
		return Mutant{Path: path, Line: tok.Line, Tier: item.tier, Operator: item.op, Original: tok.Text, Mutated: item.to, Start: tok.Start, End: tok.End}, true
	}
	if tok.Kind == "kw" || tok.Kind == "ident" {
		swap := map[string]string{"True": "False", "False": "True", "true": "false", "false": "true"}
		if to, ok := swap[tok.Text]; ok {
			return Mutant{Path: path, Line: tok.Line, Tier: 2, Operator: "boolean_substitution", Original: tok.Text, Mutated: to, Start: tok.Start, End: tok.End}, true
		}
	}
	return Mutant{}, false
}

func returnMutants(path, source, language string, lines map[int]bool) []Mutant {
	repl := "return None"
	if language != "python" {
		repl = "return undefined"
	}
	var out []Mutant
	offset := 0
	for i, line := range strings.Split(source, "\n") {
		n := i + 1
		trimmed := strings.TrimSpace(line)
		end := offset + len(line)
		if (lines == nil || lines[n]) && strings.HasPrefix(trimmed, "return ") && trimmed != "return None" && trimmed != "return undefined" && trimmed != "return 0" {
			start := offset + strings.Index(line, "return ")
			out = append(out, Mutant{Path: path, Line: n, Tier: 1, Operator: "return_value", Original: trimmed, Mutated: repl, Start: start, End: end})
		}
		offset = end + 1
	}
	return out
}

func logMutants(path, source string, lines map[int]bool) []Mutant {
	var out []Mutant
	offset := 0
	for i, line := range strings.Split(source, "\n") {
		n := i + 1
		trim := strings.TrimSpace(line)
		end := offset + len(line)
		if (lines == nil || lines[n]) && (strings.HasPrefix(trim, "console.log") || strings.HasPrefix(trim, "log.info") || strings.HasPrefix(trim, "fmt.Println")) {
			out = append(out, Mutant{Path: path, Line: n, Tier: 3, Operator: "statement_deletion", Original: trim, Mutated: "", Start: offset, End: end})
		}
		offset = end + 1
	}
	return out
}

// Exec runs a command in a directory and returns its exit code.
type Exec func(ctx context.Context, dir string, command []string) (int, error)

// Apply runs mutants in a copy of repo.
func Apply(ctx context.Context, repo string, mutants []Mutant, command []string, timeout time.Duration, max int, run Exec) (Result, error) {
	if run == nil {
		run = DefaultExec
	}
	if len(command) == 0 {
		return Result{}, fmt.Errorf("mutation test_command is empty")
	}
	sort.Slice(mutants, func(i, j int) bool {
		if mutants[i].Path != mutants[j].Path {
			return mutants[i].Path < mutants[j].Path
		}
		if mutants[i].Line != mutants[j].Line {
			return mutants[i].Line < mutants[j].Line
		}
		return mutants[i].Operator < mutants[j].Operator
	})
	res := Result{Survivors: []Mutant{}}
	if max > 0 && len(mutants) > max {
		mutants = mutants[:max]
		res.Truncated = true
	}
	dst, err := mkdirTemp("", "proofsieve-mut-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dst)
	if err := copyTree(repo, dst); err != nil {
		return Result{}, err
	}
	original := map[string][]byte{}
	for _, m := range mutants {
		switch m.Tier {
		case 1:
			res.Tier1Total++
		case 2:
			res.Tier2Total++
		default:
			res.Tier3Total++
		}
		full := filepath.Join(dst, filepath.FromSlash(m.Path))
		buf, ok := original[m.Path]
		if !ok {
			buf, err = os.ReadFile(full)
			if err != nil {
				return Result{}, err
			}
			original[m.Path] = buf
		}
		if m.Start < 0 || m.End > len(buf) || m.Start > m.End {
			return Result{}, fmt.Errorf("mutant offset out of range for %s", m.Path)
		}
		next := append([]byte{}, buf[:m.Start]...)
		next = append(next, []byte(m.Mutated)...)
		next = append(next, buf[m.End:]...)
		if err := writeFile(full, next, 0o644); err != nil {
			return Result{}, err
		}
		cctx, cancel := context.WithTimeout(ctx, timeout)
		code, runErr := run(cctx, dst, command)
		cancel()
		_ = os.WriteFile(full, buf, 0o644)
		killed := code != 0 || errors.Is(runErr, context.DeadlineExceeded)
		if runErr != nil && !errors.Is(runErr, context.DeadlineExceeded) {
			return Result{}, runErr
		}
		if killed {
			switch m.Tier {
			case 1:
				res.Tier1Killed++
			case 2:
				res.Tier2Killed++
			default:
				res.Tier3Killed++
			}
			continue
		}
		res.Survivors = append(res.Survivors, m)
	}
	return res, nil
}

// DefaultExec runs command without a shell.
func DefaultExec(ctx context.Context, dir string, command []string) (int, error) {
	if len(command) == 0 {
		return 0, fmt.Errorf("empty command")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return 1, err
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, err
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := relPath(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := d.Name()
		if d.IsDir() && (base == ".git" || base == "node_modules" || base == ".venv" || base == "venv" || base == "__pycache__") {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := readFile(path)
		if err != nil {
			return err
		}
		if err := mkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
