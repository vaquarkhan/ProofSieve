// Package behavior summarizes signature changes, calls, and callers from a diff.
package behavior

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vaquarkhan/ProofSieve/internal/diffparse"
	"github.com/vaquarkhan/ProofSieve/internal/gitx"
	"github.com/vaquarkhan/ProofSieve/internal/lang"
)

var (
	readFile = os.ReadFile
	showFile = gitx.Show
	walkDir  = filepath.WalkDir
)

// Change is an added, removed, or signature-changed function.
type Change struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// Call is a call inside a function the diff touched.
type Call struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Sensitive bool   `json:"sensitive"`
}

// Caller is an in-repo function that calls a touched function.
type Caller struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Callee string `json:"callee"`
}

// Diff is the behavioral summary for one check.
type Diff struct {
	SpecRef  string   `json:"spec_ref,omitempty"`
	Changes  []Change `json:"changes,omitempty"`
	Calls    []Call   `json:"calls,omitempty"`
	Callers  []Caller `json:"callers,omitempty"`
	Branches []string `json:"branches,omitempty"`
}

// HasBranch reports whether name is a changed function that contains a conditional.
func (d Diff) HasBranch(name string) bool {
	if name == "" {
		return false
	}
	for _, item := range d.Branches {
		if item == name {
			return true
		}
	}
	return false
}

// HasCall reports whether name is called inside a function the diff touched.
func (d Diff) HasCall(name string) bool {
	if name == "" {
		return false
	}
	for _, item := range d.Calls {
		if item.Name == name {
			return true
		}
	}
	return false
}

// DefaultSensitive is the call list used when policy does not set one.
func DefaultSensitive() []string {
	return []string{"eval", "exec", "system", "Popen", "RemoveAll", "Query", "fetch", "subprocess"}
}

// Build reads the diff and the worktree. A file that cannot be parsed is skipped.
func Build(ctx context.Context, repo, base, spec string, files []diffparse.File, sensitive []string) Diff {
	if len(sensitive) == 0 {
		sensitive = DefaultSensitive()
	}
	sens := map[string]bool{}
	for _, name := range sensitive {
		sens[name] = true
	}
	out := Diff{SpecRef: spec, Changes: []Change{}, Calls: []Call{}, Callers: []Caller{}, Branches: []string{}}
	names := map[string]bool{}
	for _, f := range files {
		path := f.Path()
		language, known := lang.Language(path)
		if !known || !lang.Supported(language) {
			continue
		}
		if f.Deleted() {
			src, ok := loadBase(ctx, repo, base, path)
			if !ok {
				continue
			}
			fns, err := lang.Analyze(language, src)
			if err != nil {
				continue
			}
			for _, fn := range fns {
				out.Changes = append(out.Changes, Change{Kind: "removed", Name: fn.Name, Path: path, Line: fn.StartLine})
				names[fn.Name] = true
			}
			continue
		}
		head, ok := loadHead(repo, path)
		if !ok {
			continue
		}
		fns, err := lang.Analyze(language, head)
		if err != nil {
			continue
		}
		toks, _ := lang.Tokens(language, head)
		baseFns := map[string]lang.Function{}
		baseSrc := ""
		if !f.AddedFile() {
			if src, ok := loadBase(ctx, repo, base, path); ok {
				baseSrc = src
				if parsed, err := lang.Analyze(language, src); err == nil {
					for _, fn := range parsed {
						baseFns[fn.Name] = fn
					}
				}
			}
		}
		touched := diffparse.NewLines(f)
		for _, fn := range fns {
			if !f.AddedFile() && !overlaps(fn, touched) {
				continue
			}
			names[fn.Name] = true
			if prev, exists := baseFns[fn.Name]; exists {
				if declLine(baseSrc, prev.StartLine) != declLine(head, fn.StartLine) {
					out.Changes = append(out.Changes, Change{Kind: "signature", Name: fn.Name, Path: path, Line: fn.StartLine})
				}
			} else {
				out.Changes = append(out.Changes, Change{Kind: "added", Name: fn.Name, Path: path, Line: fn.StartLine})
			}
			if branched(toks, fn) {
				out.Branches = append(out.Branches, fn.Name)
			}
			out.Calls = append(out.Calls, callsIn(toks, fn, path, sens)...)
		}
	}
	out.Callers = findCallers(repo, names)
	return out
}

func loadHead(repo, path string) (string, bool) {
	b, err := readFile(filepath.Join(repo, filepath.FromSlash(path)))
	if err != nil {
		return "", false
	}
	return string(b), true
}

func loadBase(ctx context.Context, repo, base, path string) (string, bool) {
	b, ok, err := showFile(ctx, repo, base, path)
	if err != nil || !ok {
		return "", false
	}
	return string(b), true
}

func skipName(name string) bool {
	return name == "" || name == "<anonymous>" || name == "<lambda>"
}

func overlaps(fn lang.Function, lines map[int]bool) bool {
	for line := fn.StartLine; line <= fn.EndLine; line++ {
		if lines[line] {
			return true
		}
	}
	return false
}

func declLine(src string, line int) string {
	parts := strings.Split(src, "\n")
	if line < 1 || line > len(parts) {
		return ""
	}
	return strings.TrimSpace(parts[line-1])
}

func branched(toks []lang.Token, fn lang.Function) bool {
	for _, tok := range toks {
		if tok.Line < fn.StartLine || tok.Line > fn.EndLine {
			continue
		}
		if tok.Kind == "kw" && (tok.Text == "if" || tok.Text == "elif" || tok.Text == "for" || tok.Text == "while" || tok.Text == "switch") {
			return true
		}
	}
	return false
}

func callsIn(toks []lang.Token, fn lang.Function, path string, sens map[string]bool) []Call {
	var out []Call
	for i, tok := range toks {
		if tok.Line < fn.StartLine || tok.Line > fn.EndLine {
			continue
		}
		if tok.Kind != "ident" && tok.Kind != "kw" {
			continue
		}
		if i+1 >= len(toks) || toks[i+1].Text != "(" {
			continue
		}
		if keyword(tok.Text) {
			continue
		}
		out = append(out, Call{Name: tok.Text, Path: path, Line: tok.Line, Sensitive: sens[tok.Text]})
	}
	return out
}

func keyword(name string) bool {
	switch name {
	case "if", "elif", "else", "for", "while", "switch", "def", "class", "func", "function", "return", "case":
		return true
	default:
		return false
	}
}

func findCallers(repo string, names map[string]bool) []Caller {
	if repo == "" || len(names) == 0 {
		return nil
	}
	var keys []string
	for name := range names {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	var out []Caller
	_ = walkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".venv", "venv", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= 20 {
			return filepath.SkipAll
		}
		language, known := lang.Language(path)
		if !known || !lang.Supported(language) {
			return nil
		}
		b, err := readFile(path)
		if err != nil {
			return nil
		}
		src := string(b)
		fns, err := lang.Analyze(language, src)
		if err != nil {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, repo))
		rel = strings.TrimPrefix(rel, "/")
		for _, name := range keys {
			line, ok := callLine(src, name)
			if !ok {
				continue
			}
			owner, ok := enclosing(fns, line)
			if !ok || owner == name {
				continue
			}
			out = append(out, Caller{Name: owner, Path: rel, Line: line, Callee: name})
		}
		return nil
	})
	return out
}

func callLine(src, name string) (int, bool) {
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, "def "+name) || strings.Contains(line, "func "+name) || strings.Contains(line, "function "+name) {
			continue
		}
		if hasCall(line, name) {
			return i + 1, true
		}
	}
	return 0, false
}

func hasCall(line, name string) bool {
	for i := 0; i+len(name) <= len(line); i++ {
		if line[i:i+len(name)] != name {
			continue
		}
		if i > 0 && isIdent(line[i-1]) {
			continue
		}
		j := i + len(name)
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		if j < len(line) && line[j] == '(' {
			return true
		}
	}
	return false
}

func isIdent(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func enclosing(fns []lang.Function, line int) (string, bool) {
	for _, fn := range fns {
		if line >= fn.StartLine && line <= fn.EndLine && !skipName(fn.Name) {
			return fn.Name, true
		}
	}
	return "", false
}
