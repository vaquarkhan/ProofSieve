package lang

import "testing"

func TestPythonComplexity(t *testing.T) {
	src := "def f(x):\n    if x > 1 and x < 10:\n        return x\n    return 0\n"
	fns, err := Analyze("python", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 1 || fns[0].Name != "f" || fns[0].Cognitive != 2 || fns[0].MaxNesting != 1 {
		t.Fatalf("%+v", fns)
	}
	nested := "def f(x):\n    if x:\n        if x:\n            return 1\n"
	fns, err = Analyze("python", nested)
	if err != nil {
		t.Fatal(err)
	}
	if fns[0].Cognitive != 3 || fns[0].MaxNesting != 2 {
		t.Fatalf("nested %+v", fns[0])
	}
	async := "async def run(x):\n    return x\n"
	fns, err = Analyze("python", async)
	if err != nil || len(fns) != 1 || fns[0].Name != "run" {
		t.Fatalf("async %+v %v", fns, err)
	}
	one := "def f():\n    return 1\n"
	fns, err = Analyze("python", one)
	if err != nil || fns[0].Lines < 2 {
		t.Fatalf("lines %+v %v", fns, err)
	}
}

func TestPythonStringsAndErrors(t *testing.T) {
	src := "def f(x):\n    s = 'if and or'\n    t = \"\"\"line\nstill\"\"\"\n    return s\n"
	if _, err := Analyze("python", src); err != nil {
		t.Fatal(err)
	}
	if _, err := Analyze("python", "def f():\n    return 'nope\n"); err == nil {
		t.Fatal("wanted unterminated string")
	}
	if _, err := Analyze("python", "def f():\n    x = 1\n  y = 2\n"); err == nil {
		t.Fatal("wanted bad indent")
	}
	if _, err := Analyze("python", "def f():\n    return )\n"); err == nil {
		t.Fatal("wanted unmatched paren")
	}
	toks, err := Tokens("python", "x = 1 # if\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range toks {
		if tok.Text == "if" {
			t.Fatal("comment leaked")
		}
	}
}

func TestCLike(t *testing.T) {
	src := "function f(x) {\n  if (x > 1 && x < 3) {\n    return x;\n  }\n  return 0;\n}\n"
	fns, err := Analyze("typescript", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 1 || fns[0].Name != "f" || fns[0].Cognitive != 2 {
		t.Fatalf("%+v", fns)
	}
	nested := "function f(x) {\n  if (x) {\n    if (x) { return 1; }\n  } else {\n    return 2;\n  }\n}\n"
	fns, err = Analyze("javascript", nested)
	if err != nil {
		t.Fatal(err)
	}
	if fns[0].Cognitive < 3 || fns[0].MaxNesting < 2 {
		t.Fatalf("nested %+v", fns[0])
	}
	arrow := "const f = (x) => {\n  return x;\n};\n"
	fns, err = Analyze("typescript", arrow)
	if err != nil || len(fns) != 1 {
		t.Fatalf("arrow %+v %v", fns, err)
	}
	goSrc := "package p\nfunc F(x int) int {\n  if x > 0 {\n    return x\n  }\n  return 0\n}\n"
	fns, err = Analyze("go", goSrc)
	if err != nil || len(fns) != 1 || fns[0].Name != "F" || fns[0].Cognitive != 1 {
		t.Fatalf("go %+v %v", fns, err)
	}
	if _, err = Analyze("typescript", "function f(x) { if (x) { return "); err == nil {
		t.Fatal("wanted unclosed")
	}
	if _, err = Analyze("typescript", "function f() { return `no"); err == nil {
		t.Fatal("wanted string")
	}
	if _, err = Analyze("go", "func F() { /* "); err == nil {
		t.Fatal("wanted comment")
	}
	if _, err = Analyze("rust", "fn main() {}"); err == nil {
		t.Fatal("rust has no analyzer")
	}
	if _, err = Tokens("rust", "fn"); err == nil {
		t.Fatal("rust tokens")
	}
}

func TestLanguageAndTests(t *testing.T) {
	lang, ok := Language("a/b.TSX")
	if !ok || lang != "typescript" {
		t.Fatalf("%s %v", lang, ok)
	}
	if _, ok = Language("README.md"); ok {
		t.Fatal("md")
	}
	if !Supported("go") || Supported("java") {
		t.Fatal("supported")
	}
	if !IsTest("billing/oracle/test_rounding.py") || !IsTest("pkg/a_test.go") || !IsTest("web/a.spec.ts") {
		t.Fatal("test paths")
	}
	if IsTest("billing/rounding.py") {
		t.Fatal("prod path")
	}
}

func TestLogicSwitch(t *testing.T) {
	src := "function f(a, b, c) {\n  if (a && b || c) { return 1; }\n  return 0;\n}\n"
	fns, err := Analyze("typescript", src)
	if err != nil {
		t.Fatal(err)
	}
	// if +1, && sequence +1, operator switch +1
	if fns[0].Cognitive != 3 {
		t.Fatalf("cog %d", fns[0].Cognitive)
	}
}

func TestMethodNotIf(t *testing.T) {
	src := "function outer() {\n  const x = { a: 1 };\n  if (x.a) { return x.a; }\n  return 0;\n}\n"
	fns, err := Analyze("typescript", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 1 || fns[0].Name != "outer" || fns[0].Cognitive != 1 {
		t.Fatalf("%+v", fns)
	}
}
