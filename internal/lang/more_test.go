package lang

import "testing"

func TestLanguageEdges(t *testing.T) {
	for _, path := range []string{"a.py", "a.ts", "a.tsx", "a.js", "a.jsx", "a.mjs", "a.cjs", "a.go", "a.java", "a.kt", "a.kts", "a.rs", "a.txt"} {
		Language(path)
	}
	if _, err := Analyze("java", "class A {}"); err == nil {
		t.Fatal("java")
	}
	if _, err := Tokens("rust", "fn main() {}"); err == nil {
		t.Fatal("rust tokens")
	}
	if !IsTest("pkg/tests/a.py") || !IsTest("a_test.py") || !IsTest("a_test.go") || !IsTest("a.test.ts") || !IsTest("a.spec.js") || IsTest("billing/price.py") {
		t.Fatal("istest")
	}
}

func TestPythonEdges(t *testing.T) {
	samples := []string{
		"\n# comment\n",
		"    \n",
		"x = 1\r\ny = 2\n",
		"def f():\n\treturn 1\n",
		"'''abc'''\n",
		"f\"hi\"\n",
		"class A:\n    pass\n",
		"async def f():\n    return 1\n",
		"async\n",
		"def f(): return 1\n",
		"def f(x):\n    if x and y or z:\n        return x\n    elif x:\n        return 0\n    else:\n        return 1\n",
		"def (\n    pass\n",
	}
	for _, src := range samples {
		_, _ = Tokens("python", src)
		_, _ = Analyze("python", src)
	}
	if _, err := Tokens("python", "def f(:\n    `\n"); err == nil {
		_, err = Tokens("python", "`\n")
		if err == nil {
			t.Fatal("backtick")
		}
	}
	if _, err := Tokens("python", "\xff\n"); err == nil {
		t.Fatal("rune")
	}
	if _, err := Tokens("python", "x = '\\"); err == nil {
		t.Fatal("escape")
	}
	if _, err := Tokens("python", "(\n)"); err != nil {
		t.Fatal(err)
	}
	if _, err := Tokens("python", ")\n"); err == nil {
		t.Fatal("unmatched")
	}
	_, _, _, err := pythonString([]byte("f"), 0, 1)
	if err == nil {
		t.Fatal("prefix")
	}
	_, _, _, err = pythonString([]byte("uX"), 0, 1)
	if err == nil {
		t.Fatal("bad quote")
	}
	if _, _, err := pythonIndent([]byte(" \t"), 0); err != nil {
		t.Fatal(err)
	}
}

func TestCEdges(t *testing.T) {
	samples := []string{
		"// comment\nfunction f(x) { if (x && y || z) { return x } else { return 0 } }\n",
		"/* block\n comment */\nconst n = 0x1F;\n",
		"function outer() { function inner() { if (x) { return 1 } } return inner }\n",
		"() => { return 1 }\n",
		"const value = `a\nb`;\n",
		"function f(a()) { return 1 }\n",
		"{ let x = 1 }\n",
		"if (x) { return 1 }\n",
		"func F() int { if true { return 1 } }\n",
		"函数\n",
		"=== !== ... >>= <<=\n",
	}
	for _, src := range samples {
		_, _ = Tokens("typescript", src)
		_, _ = Analyze("typescript", src)
		_, _ = Analyze("go", src)
	}
	if _, err := Tokens("go", "/*"); err == nil {
		t.Fatal("comment")
	}
	if _, err := Tokens("go", "\"unterminated\n"); err == nil {
		t.Fatal("string")
	}
	if _, err := Tokens("go", "\"\\"); err == nil {
		t.Fatal("escape")
	}
	if _, err := Tokens("go", "\"\\n\""); err != nil {
		t.Fatal(err)
	}
	if _, err := Tokens("python", "    "); err != nil {
		t.Fatal(err)
	}
	if _, err := Tokens("python", "+"); err != nil {
		t.Fatal(err)
	}
	if _, err := Tokens("python", "x = '\\"); err == nil {
		t.Fatal("py escape")
	}
	toks := []Token{
		{Kind: "punct", Text: ")"},
		{Kind: "punct", Text: "("},
		{Kind: "punct", Text: ")"},
		{Kind: "punct", Text: "{"},
	}
	if startsFn(toks, 3, cConfig{funcKw: []string{"func"}}) {
		t.Fatal("paren")
	}
	if isFuncKw("if", []string{"func"}) {
		t.Fatal("kw")
	}
	if fnName([]Token{{Kind: "punct", Text: ")"}, {Kind: "punct", Text: "{"}}, 1) != "<anonymous>" {
		t.Fatal("anon")
	}
	if fnStart([]Token{{Kind: "punct", Text: ";"}, {Kind: "punct", Text: "{"}}, 1) != 1 {
		t.Fatal("start")
	}
	if _, err := Analyze("typescript", "function f() {"); err == nil {
		t.Fatal("brace")
	}
	if _, err := Analyze("go", "}"); err == nil {
		t.Fatal("close")
	}
	if _, _, _, err := cString([]byte("\""), 0, 1); err == nil {
		t.Fatal("empty string")
	}
	arrow := []Token{{Kind: "punct", Text: ")"}, {Kind: "op", Text: "=>"}, {Kind: "punct", Text: "{"}}
	if fnName(arrow, 2) != "<anonymous>" {
		t.Fatal(fnName(toks, 2))
	}
	if startsFn([]Token{{Kind: "punct", Text: ")"}, {Kind: "punct", Text: "{"}}, 1, cConfig{}) {
		t.Fatal("open")
	}
	if _, err := Analyze("go", "func() int { return 1 }\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := Tokens("python", "'\\n'"); err != nil {
		t.Fatal(err)
	}
	if pythonOp([]byte("=="), 0) != "==" || pythonOp([]byte("+"), 0) != "+" {
		t.Fatal("op")
	}
	if _, _, _, err := pythonString([]byte("'\\"), 0, 1); err == nil {
		t.Fatal("string")
	}
	if _, err := Tokens("python", "def f():\n    return 1\n  x = 1\n"); err == nil {
		t.Fatal("indent")
	}
	if startsFn([]Token{{Kind: "punct", Text: "{"}}, 0, cConfig{arrows: true}) {
		t.Fatal("bare brace")
	}
}
