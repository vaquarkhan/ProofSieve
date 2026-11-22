package behavior

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/diffparse"
	"github.com/vaquarkhan/ProofSieve/internal/lang"
)

func TestHelpers(t *testing.T) {
	if declLine("a", 0) != "" || declLine("a", 3) != "" || declLine("a\nb", 2) != "b" {
		t.Fatal(declLine("a\nb", 2))
	}
	if !skipName("") || !skipName("<anonymous>") || !skipName("<lambda>") || skipName("price") {
		t.Fatal("name")
	}
	if overlaps(lang.Function{StartLine: 1, EndLine: 2}, map[int]bool{4: true}) {
		t.Fatal("overlap")
	}
	if (Diff{}).HasBranch("") || (Diff{Calls: []Call{{Name: "eval"}}}).HasCall("missing") {
		t.Fatal("lookup")
	}
	if !keyword("return") || keyword("eval") {
		t.Fatal("keyword")
	}
	empty := Build(context.Background(), "", "HEAD", "specs/a.md", nil, nil)
	if empty.SpecRef != "specs/a.md" || empty.HasBranch("price") || empty.HasCall("") || len(empty.Callers) != 0 {
		t.Fatalf("%+v", empty)
	}
	if !(Diff{Branches: []string{"price"}, Calls: []Call{{Name: "eval"}}}).HasBranch("price") {
		t.Fatal("branch")
	}
	if !(Diff{Calls: []Call{{Name: "eval"}}}).HasCall("eval") {
		t.Fatal("call")
	}
}

func TestBuild(t *testing.T) {
	repo := t.TempDir()
	write(t, repo, "billing/price.py", "def quiet():\n    return 1\n\ndef price(x, y):\n    if (x > 0):\n        return eval(x)\n    return 0\n")
	write(t, repo, "billing/fresh.py", "def extra():\n    return helper()\n")
	write(t, repo, "billing/body.py", "def body(x):\n    if x > 0:\n        return helper(x)\n    return 0\n")
	write(t, repo, "billing/bad.py", "def f():\n    return 'nope\n")
	write(t, repo, "billing/recur.py", "def recur():\n    recur()\n")
	write(t, repo, "billing/use.py", "def use():\n    xprice()\n    price\t()\n    price + 1\n")
	write(t, repo, "billing/quiet.py", "def quiet():\n    return 1\nx = 0\n\n\n\n\n\n\n\n\n\n\n\nx = 1\n")
	write(t, repo, "billing/noname.py", "def:\n    return 1\n")
	write(t, repo, "billing/mod.py", "price()\n")
	write(t, repo, "billing/badcall.py", "def f():\n    return 'nope\n")
	write(t, repo, "billing/other.py", "def other():\n    xtarget()\n    target + 1\n")
	write(t, repo, "notes/a.md", "note\n")
	write(t, repo, "src/A.java", "class A {}\n")
	for _, dir := range []string{".git", "node_modules", ".venv", "venv", "__pycache__"} {
		write(t, repo, dir+"/skip.py", "def skip():\n    price()\n")
	}
	origShow := showFile
	origRead := readFile
	t.Cleanup(func() {
		showFile = origShow
		readFile = origRead
	})
	showFile = func(_ context.Context, _, _, path string) ([]byte, bool, error) {
		switch path {
		case "billing/price.py":
			return []byte("def quiet():\n    return 1\n\ndef price(x):\n    return x\n"), true, nil
		case "billing/body.py":
			return []byte("def body(x):\n    return x\n"), true, nil
		case "billing/gone.py":
			return []byte("def gone():\n    return 1\n"), true, nil
		case "billing/basebad.py":
			return []byte("def f():\n    return 'nope\n"), true, nil
		case "billing/missing-base.py":
			return nil, false, nil
		case "billing/show-err.py":
			return nil, false, errors.New("show")
		case "billing/gonebad.py":
			return []byte("def f():\n    return 'nope\n"), true, nil
		case "billing/noname.py":
			return []byte("def:\n    return 1\n"), true, nil
		default:
			return nil, false, nil
		}
	}
	readFile = func(path string) ([]byte, error) {
		if strings.HasSuffix(filepath.ToSlash(path), "billing/unreadable.py") {
			return nil, errors.New("read")
		}
		return origRead(path)
	}
	files := []diffparse.File{
		{OldPath: "notes/a.md", NewPath: "notes/a.md", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 1}}},
		{OldPath: "src/A.java", NewPath: "src/A.java", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 1}}},
		{OldPath: "billing/missing.py", NewPath: "billing/missing.py", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 1}}},
		{OldPath: "billing/bad.py", NewPath: "billing/bad.py", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 1}}},
		{OldPath: "billing/gone.py", NewPath: "/dev/null"},
		{OldPath: "billing/missing-base.py", NewPath: "/dev/null"},
		{OldPath: "billing/show-err.py", NewPath: "/dev/null"},
		{OldPath: "billing/gonebad.py", NewPath: "/dev/null"},
		{OldPath: "billing/quiet.py", NewPath: "billing/quiet.py", Hunks: []diffparse.Hunk{{NewStart: 16, NewLines: 1, NewText: "x = 2\n"}}},
		{OldPath: "billing/noname.py", NewPath: "billing/noname.py", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 2, NewText: "def:\n    return 1\n"}}},
		{OldPath: "billing/basebad.py", NewPath: "billing/basebad.py", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 2, NewText: "def kept():\n    return 1\n"}}},
		{OldPath: "billing/price.py", NewPath: "billing/price.py", Hunks: []diffparse.Hunk{{NewStart: 4, NewLines: 4, NewText: "def price(x, y):\n    if (x > 0):\n        return eval(x)\n    return 0\n"}}},
		{OldPath: "/dev/null", NewPath: "billing/fresh.py", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 2, NewText: "def extra():\n    return helper()\n"}}},
		{OldPath: "billing/body.py", NewPath: "billing/body.py", Hunks: []diffparse.Hunk{{NewStart: 2, NewLines: 2, NewText: "    if x > 0:\n        return helper(x)\n"}}},
		{OldPath: "/dev/null", NewPath: "billing/recur.py"},
		{OldPath: "billing/unreadable.py", NewPath: "billing/unreadable.py", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 1}}},
	}
	write(t, repo, "billing/basebad.py", "def kept():\n    return 1\n")
	got := Build(context.Background(), repo, "base", "specs/a.md", files, nil)
	if !hasKind(got, "signature", "price") || !hasKind(got, "added", "extra") || !hasKind(got, "removed", "gone") || !hasKind(got, "added", "kept") {
		t.Fatalf("changes %+v", got.Changes)
	}
	if hasKind(got, "signature", "body") || hasKind(got, "added", "quiet") {
		t.Fatalf("unexpected %+v", got.Changes)
	}
	if !got.HasBranch("price") || !got.HasBranch("body") || !got.HasCall("eval") || !got.HasCall("helper") {
		t.Fatalf("facts %+v %+v", got.Branches, got.Calls)
	}
	sens := false
	for _, c := range got.Calls {
		if c.Name == "eval" && c.Sensitive {
			sens = true
		}
	}
	if !sens {
		t.Fatal("sensitive")
	}
	if !hasCaller(got, "use", "price") {
		t.Fatalf("callers %+v", got.Callers)
	}
	narrow := Build(context.Background(), repo, "base", "specs/a.md", []diffparse.File{files[11]}, []string{"system"})
	found := false
	for _, c := range narrow.Calls {
		if c.Name == "eval" {
			found = true
			if c.Sensitive {
				t.Fatal("custom list")
			}
		}
	}
	if !found {
		t.Fatal("eval call")
	}
}

func TestCallerEdges(t *testing.T) {
	repo := t.TempDir()
	write(t, repo, "lib.py", "def target():\n    return 1\n")
	for i := 0; i < 21; i++ {
		write(t, repo, "c"+two(i)+".py", "def caller():\n    target()\n")
	}
	for _, dir := range []string{".git", "node_modules", ".venv", "venv", "__pycache__"} {
		write(t, repo, dir+"/skip.py", "def skip():\n    target()\n")
	}
	files := []diffparse.File{{OldPath: "/dev/null", NewPath: "lib.py", Hunks: []diffparse.Hunk{{NewStart: 1, NewLines: 2}}}}
	got := Build(context.Background(), repo, "base", "", files, []string{"eval"})
	if len(got.Callers) != 20 {
		t.Fatalf("cap %d", len(got.Callers))
	}
	orig := walkDir
	walkDir = func(root string, fn fs.WalkDirFunc) error {
		return fn(root, nil, errors.New("walk"))
	}
	t.Cleanup(func() { walkDir = orig })
	if callers := findCallers(repo, map[string]bool{"target": true}); callers != nil {
		t.Fatal(callers)
	}
	walkDir = orig
	origRead := readFile
	readFile = func(string) ([]byte, error) { return nil, errors.New("read") }
	t.Cleanup(func() { readFile = origRead })
	if len(findCallers(repo, map[string]bool{"target": true})) != 0 {
		t.Fatal("read")
	}
}

func hasKind(d Diff, kind, name string) bool {
	for _, c := range d.Changes {
		if c.Kind == kind && c.Name == name {
			return true
		}
	}
	return false
}

func hasCaller(d Diff, name, callee string) bool {
	for _, c := range d.Callers {
		if c.Name == name && c.Callee == callee {
			return true
		}
	}
	return false
}

func two(n int) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}

func write(t *testing.T, repo, rel, body string) {
	t.Helper()
	full := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
