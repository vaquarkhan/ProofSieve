package lang

import "fmt"

type cConfig struct {
	funcKw []string
	arrows bool
}

func lexC(src string) ([]Token, error) {
	var out []Token
	b := []byte(src)
	i, line := 0, 1
	for i < len(b) {
		c := b[i]
		if c == ' ' || c == '\t' || c == '\r' {
			i++
			continue
		}
		if c == '\n' {
			i++
			line++
			continue
		}
		if c == '/' && i+1 < len(b) && b[i+1] == '/' {
			for i < len(b) && b[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(b) && b[i+1] == '*' {
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				if b[i] == '\n' {
					line++
				}
				i++
			}
			if i+1 >= len(b) {
				return nil, fmt.Errorf("line %d: unterminated block comment", line)
			}
			i += 2
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			tok, ni, nline, err := cString(b, i, line)
			if err != nil {
				return nil, err
			}
			out = append(out, tok)
			i, line = ni, nline
			continue
		}
		if isIdentStart(rune(c)) || c >= 0x80 {
			start := i
			r, w := utf8Decode(b[i:])
			i += w
			for i < len(b) {
				r, w = utf8Decode(b[i:])
				if !isIdentCont(r) {
					break
				}
				i += w
			}
			text := string(b[start:i])
			kind := "ident"
			if cKeyword(text) {
				kind = "kw"
			}
			out = append(out, Token{Kind: kind, Text: text, Line: line, Start: start, End: i})
			continue
		}
		if c >= '0' && c <= '9' {
			start := i
			for i < len(b) && (b[i] == '.' || b[i] == '_' || b[i] == 'x' || b[i] == 'X' || (b[i] >= '0' && b[i] <= '9') || (b[i] >= 'a' && b[i] <= 'f') || (b[i] >= 'A' && b[i] <= 'F')) {
				i++
			}
			out = append(out, Token{Kind: "num", Text: string(b[start:i]), Line: line, Start: start, End: i})
			continue
		}
		start := i
		op := cOp(b, i)
		i += len(op)
		kind := "op"
		if op == "(" || op == ")" || op == "[" || op == "]" || op == "{" || op == "}" || op == ";" || op == "," || op == "." || op == ":" {
			kind = "punct"
		}
		out = append(out, Token{Kind: kind, Text: op, Line: line, Start: start, End: i})
	}
	return out, nil
}

func utf8Decode(b []byte) (rune, int) {
	r, w := rune(b[0]), 1
	if b[0] < 0x80 {
		return r, w
	}
	r, w = decodeRune(b)
	return r, w
}

func decodeRune(b []byte) (rune, int) {
	// local wrapper so python.go's unicode import is the only unicode use site
	return runeFrom(b)
}

func cString(b []byte, i, line int) (Token, int, int, error) {
	start := i
	q := b[i]
	i++
	for i < len(b) {
		if b[i] == '\\' && q != '`' {
			i += 2
			if i > len(b) {
				break
			}
			continue
		}
		if b[i] == '\n' {
			if q != '`' {
				return Token{}, 0, 0, fmt.Errorf("line %d: unterminated string", line)
			}
			line++
		}
		if b[i] == q {
			i++
			return Token{Kind: "str", Text: string(b[start:i]), Line: line, Start: start, End: i}, i, line, nil
		}
		i++
	}
	return Token{}, 0, 0, fmt.Errorf("line %d: unterminated string", line)
}

func cKeyword(s string) bool {
	switch s {
	case "if", "else", "for", "while", "switch", "case", "catch", "do", "function", "func",
		"return", "true", "false", "null", "undefined", "class", "interface", "try", "finally",
		"break", "continue", "new", "var", "let", "const", "type", "struct", "import", "package",
		"defer", "go", "select", "range", "map", "chan", "and", "or":
		return true
	default:
		return false
	}
}

func cOp(b []byte, i int) string {
	if i+2 < len(b) {
		three := string(b[i : i+3])
		switch three {
		case "===", "!==", "...", ">>=", "<<=":
			return three
		}
	}
	if i+1 < len(b) {
		two := string(b[i : i+2])
		switch two {
		case "==", "!=", "<=", ">=", "&&", "||", "++", "--", "+=", "-=", "*=", "/=", "=>", "->", "<<", ">>", ":=":
			return two
		}
	}
	return string(b[i])
}

func analyzeC(src string, cfg cConfig) ([]Function, error) {
	toks, err := lexC(src)
	if err != nil {
		return nil, err
	}
	ranges, err := cFunctions(toks, cfg)
	if err != nil {
		return nil, err
	}
	var fns []Function
	for i, rg := range ranges {
		cog, nest := scoreC(toks, rg, ranges, i)
		fns = append(fns, Function{
			Name:       rg.name,
			StartLine:  toks[rg.start].Line,
			EndLine:    toks[rg.end-1].Line,
			Cognitive:  cog,
			MaxNesting: nest,
			Lines:      toks[rg.end-1].Line - toks[rg.start].Line + 1,
		})
	}
	return fns, nil
}

type cRange struct {
	name       string
	start, end int
	body       int
}

func cFunctions(toks []Token, cfg cConfig) ([]cRange, error) {
	var fns []cRange
	type frame struct{ idx, depth int }
	var stack []frame
	brace := 0
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.Kind == "punct" && t.Text == "{" {
			if startsFn(toks, i, cfg) {
				stack = append(stack, frame{idx: len(fns), depth: brace})
				fns = append(fns, cRange{name: fnName(toks, i), start: fnStart(toks, i), body: i, end: -1})
			}
			brace++
			continue
		}
		if t.Kind == "punct" && t.Text == "}" {
			if brace == 0 {
				return nil, fmt.Errorf("line %d: unmatched }", t.Line)
			}
			brace--
			if n := len(stack); n > 0 && stack[n-1].depth == brace {
				fns[stack[n-1].idx].end = i + 1
				stack = stack[:n-1]
			}
		}
	}
	if brace != 0 {
		return nil, fmt.Errorf("unbalanced braces")
	}
	return fns, nil
}

func startsFn(toks []Token, braceAt int, cfg cConfig) bool {
	j := braceAt - 1
	for j >= 0 && (toks[j].Kind == "ident" || toks[j].Text == "*" || toks[j].Text == "." || toks[j].Text == "[" || toks[j].Text == "]") {
		j--
	}
	if j < 0 {
		return false
	}
	if cfg.arrows && toks[j].Text == "=>" {
		return true
	}
	if toks[j].Text != ")" {
		return false
	}
	depth := 1
	j--
	for j >= 0 && depth > 0 {
		if toks[j].Text == ")" {
			depth++
		}
		if toks[j].Text == "(" {
			depth--
		}
		j--
	}
	if j < 0 {
		return false
	}
	if isFuncKw(toks[j].Text, cfg.funcKw) {
		return true
	}
	return toks[j].Kind == "ident"
}

func isFuncKw(text string, kws []string) bool {
	for _, k := range kws {
		if text == k {
			return true
		}
	}
	return false
}

func fnName(toks []Token, braceAt int) string {
	j := braceAt - 1
	for j >= 0 && (toks[j].Kind == "ident" || toks[j].Text == "*" || toks[j].Text == "." || toks[j].Text == "[" || toks[j].Text == "]") {
		j--
	}
	if j >= 0 && toks[j].Text == "=>" {
		for ; j >= 0; j-- {
			if toks[j].Text == "=" && j > 0 && toks[j-1].Kind == "ident" {
				return toks[j-1].Text
			}
		}
		return "<anonymous>"
	}
	if j >= 0 && toks[j].Text == ")" {
		depth := 1
		j--
		for j >= 0 && depth > 0 {
			if toks[j].Text == ")" {
				depth++
			}
			if toks[j].Text == "(" {
				depth--
			}
			j--
		}
		if j >= 0 && toks[j].Kind == "ident" {
			return toks[j].Text
		}
	}
	return "<anonymous>"
}

func fnStart(toks []Token, braceAt int) int {
	start := braceAt
	for j := braceAt; j >= 0; j-- {
		start = j
		if toks[j].Kind == "kw" && (toks[j].Text == "function" || toks[j].Text == "func") {
			return j
		}
		if j < braceAt && toks[j].Kind == "punct" && (toks[j].Text == ";" || toks[j].Text == "}") {
			return j + 1
		}
	}
	return start
}

func scoreC(toks []Token, rg cRange, all []cRange, self int) (int, int) {
	cog, nest, maxN, brace := 0, 0, 0, 0
	pending := false
	var opened []int
	prevLogic := ""
	logicLive := false
	for i := rg.body; i < rg.end; i++ {
		if insideHole(all, self, i) {
			continue
		}
		t := toks[i]
		if t.Kind == "punct" && t.Text == "{" {
			brace++
			if pending {
				nest++
				if nest > maxN {
					maxN = nest
				}
				opened = append(opened, brace)
				pending = false
			}
			continue
		}
		if t.Kind == "punct" && t.Text == "}" {
			brace--
			for len(opened) > 0 && opened[len(opened)-1] > brace {
				opened = opened[:len(opened)-1]
				if nest > 0 {
					nest--
				}
			}
			pending = false
			logicLive = false
			prevLogic = ""
			continue
		}
		if t.Kind == "punct" && t.Text == ";" {
			pending = false
			logicLive = false
			prevLogic = ""
			continue
		}
		if t.Kind == "kw" && cControl(t.Text) {
			cog += 1 + nest
			pending = t.Text != "else" && t.Text != "case"
			logicLive = false
			prevLogic = ""
			continue
		}
		if t.Text == "&&" || t.Text == "||" {
			if !logicLive {
				cog++
				logicLive = true
				prevLogic = t.Text
			} else if prevLogic != t.Text {
				cog++
				prevLogic = t.Text
			}
		}
	}
	return cog, maxN
}

func insideHole(all []cRange, self, i int) bool {
	for n, other := range all {
		if n == self {
			continue
		}
		if i > other.body && i < other.end-1 && other.body > all[self].body && other.end < all[self].end {
			return true
		}
	}
	return false
}

func cControl(s string) bool {
	switch s {
	case "if", "else", "for", "while", "switch", "case", "catch", "do":
		return true
	default:
		return false
	}
}
