package lang

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

func lexPython(src string) ([]Token, error) {
	var out []Token
	b := []byte(src)
	i, line := 0, 1
	bol := true
	paren := 0
	indents := []int{0}
	for i < len(b) {
		if bol && paren == 0 {
			bol = false
			width, next, _ := pythonIndent(b, i)
			i = next
			if i >= len(b) {
				break
			}
			if b[i] == '\n' || b[i] == '\r' || b[i] == '#' {
				// blank or comment line does not change indent
			} else {
				top := indents[len(indents)-1]
				if width > top {
					indents = append(indents, width)
					out = append(out, Token{Kind: "indent", Line: line, Start: i, End: i})
				}
				for width < indents[len(indents)-1] {
					indents = indents[:len(indents)-1]
					out = append(out, Token{Kind: "dedent", Line: line, Start: i, End: i})
				}
				if width != indents[len(indents)-1] {
					return nil, fmt.Errorf("python line %d: indent does not match an outer block", line)
				}
			}
		}
		c := b[i]
		if c == ' ' || c == '\t' {
			i++
			continue
		}
		if c == '#' {
			for i < len(b) && b[i] != '\n' {
				i++
			}
			continue
		}
		if c == '\r' {
			i++
			continue
		}
		if c == '\n' {
			if paren == 0 {
				out = append(out, Token{Kind: "nl", Text: "\n", Line: line, Start: i, End: i + 1})
			}
			i++
			line++
			bol = true
			continue
		}
		bol = false
		if c == '\'' || c == '"' || (c == 'f' || c == 'F' || c == 'r' || c == 'R' || c == 'b' || c == 'B') && i+1 < len(b) && (b[i+1] == '\'' || b[i+1] == '"') {
			tok, ni, nline, err := pythonString(b, i, line)
			if err != nil {
				return nil, err
			}
			out = append(out, tok)
			i, line = ni, nline
			continue
		}
		if isIdentStart(rune(c)) || c >= 0x80 {
			start := i
			r, w := utf8.DecodeRune(b[i:])
			if !isIdentStart(r) {
				return nil, fmt.Errorf("python line %d: bad rune", line)
			}
			i += w
			for i < len(b) {
				r, w = utf8.DecodeRune(b[i:])
				if !isIdentCont(r) {
					break
				}
				i += w
			}
			text := string(b[start:i])
			kind := "ident"
			if pythonKeyword(text) {
				kind = "kw"
			}
			out = append(out, Token{Kind: kind, Text: text, Line: line, Start: start, End: i})
			continue
		}
		if c >= '0' && c <= '9' {
			start := i
			for i < len(b) && (b[i] == '.' || b[i] == '_' || (b[i] >= '0' && b[i] <= '9') || b[i] == 'x' || b[i] == 'X' || (b[i] >= 'a' && b[i] <= 'f') || (b[i] >= 'A' && b[i] <= 'F')) {
				i++
			}
			out = append(out, Token{Kind: "num", Text: string(b[start:i]), Line: line, Start: start, End: i})
			continue
		}
		if isPythonOpStart(c) {
			start := i
			op := pythonOp(b, i)
			i += len(op)
			kind := "op"
			if op == "(" || op == ")" || op == "[" || op == "]" || op == "{" || op == "}" || op == ":" || op == "," || op == "." {
				kind = "punct"
			}
			if op == "(" || op == "[" || op == "{" {
				paren++
			}
			if op == ")" || op == "]" || op == "}" {
				if paren == 0 {
					return nil, fmt.Errorf("python line %d: unmatched %s", line, op)
				}
				paren--
			}
			out = append(out, Token{Kind: kind, Text: op, Line: line, Start: start, End: i})
			continue
		}
		return nil, fmt.Errorf("python line %d: unexpected %q", line, string(c))
	}
	for len(indents) > 1 {
		indents = indents[:len(indents)-1]
		out = append(out, Token{Kind: "dedent", Line: line, Start: len(b), End: len(b)})
	}
	return out, nil
}

func pythonIndent(b []byte, i int) (int, int, error) {
	w := 0
	for i < len(b) {
		switch b[i] {
		case ' ':
			w++
			i++
		case '\t':
			w += 4
			i++
		default:
			return w, i, nil
		}
	}
	return w, i, nil
}

func pythonString(b []byte, i, line int) (Token, int, int, error) {
	start := i
	for i < len(b) && (b[i] == 'f' || b[i] == 'F' || b[i] == 'r' || b[i] == 'R' || b[i] == 'b' || b[i] == 'B' || b[i] == 'u' || b[i] == 'U') {
		i++
	}
	if i >= len(b) {
		return Token{}, 0, 0, fmt.Errorf("python line %d: unfinished string", line)
	}
	q := b[i]
	if q != '\'' && q != '"' {
		return Token{}, 0, 0, fmt.Errorf("python line %d: unfinished string", line)
	}
	triple := i+2 < len(b) && b[i+1] == q && b[i+2] == q
	if triple {
		i += 3
	} else {
		i++
	}
	for i < len(b) {
		if b[i] == '\\' {
			i += 2
			if i > len(b) {
				break
			}
			continue
		}
		if b[i] == '\n' {
			line++
		}
		if triple && i+2 < len(b) && b[i] == q && b[i+1] == q && b[i+2] == q {
			i += 3
			return Token{Kind: "str", Text: string(b[start:i]), Line: line, Start: start, End: i}, i, line, nil
		}
		if !triple && b[i] == q {
			i++
			return Token{Kind: "str", Text: string(b[start:i]), Line: line, Start: start, End: i}, i, line, nil
		}
		if !triple && b[i] == '\n' {
			return Token{}, 0, 0, fmt.Errorf("python line %d: unterminated string", line)
		}
		i++
	}
	return Token{}, 0, 0, fmt.Errorf("python line %d: unterminated string", line)
}

func pythonKeyword(s string) bool {
	switch s {
	case "and", "or", "not", "if", "elif", "else", "for", "while", "try", "except", "finally",
		"with", "match", "case", "def", "class", "return", "async", "await", "lambda",
		"True", "False", "None", "in", "is", "pass", "raise", "yield", "break", "continue",
		"import", "from", "as", "assert", "global", "nonlocal", "del":
		return true
	default:
		return false
	}
}

func isPythonOpStart(c byte) bool {
	switch c {
	case '+', '-', '*', '/', '%', '=', '!', '<', '>', '&', '|', '^', '~', '@', ':', ',', '.', '(', ')', '[', ']', '{', '}', ';':
		return true
	default:
		return false
	}
}

func pythonOp(b []byte, i int) string {
	two := ""
	if i+1 < len(b) {
		two = string(b[i : i+2])
	}
	switch two {
	case "==", "!=", "<=", ">=", "//", "**", "+=", "-=", "*=", "/=", "->", ":=", "<<", ">>":
		return two
	}
	return string(b[i])
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentCont(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func analyzePython(src string) ([]Function, error) {
	toks, err := lexPython(src)
	if err != nil {
		return nil, err
	}
	var fns []Function
	for i := 0; i < len(toks); i++ {
		if toks[i].Kind != "kw" || (toks[i].Text != "def" && toks[i].Text != "async") {
			continue
		}
		j := i
		if toks[i].Text == "async" {
			if i+1 >= len(toks) || toks[i+1].Text != "def" {
				continue
			}
			j = i + 1
		}
		name := "<lambda>"
		if j+1 < len(toks) && toks[j+1].Kind == "ident" {
			name = toks[j+1].Text
		}
		end := pythonFuncEnd(toks, j)
		body := toks[j:end]
		cog, nest := scorePython(body)
		fns = append(fns, Function{
			Name:       name,
			StartLine:  toks[i].Line,
			EndLine:    toks[end-1].Line,
			Cognitive:  cog,
			MaxNesting: nest,
			Lines:      toks[end-1].Line - toks[i].Line + 1,
		})
		i = end - 1
	}
	return fns, nil
}

func pythonFuncEnd(toks []Token, defAt int) int {
	depth := 0
	started := false
	sawColon := false
	for i := defAt; i < len(toks); i++ {
		if toks[i].Kind == "punct" && toks[i].Text == ":" {
			sawColon = true
		}
		switch toks[i].Kind {
		case "indent":
			depth++
			started = true
		case "dedent":
			depth--
			if started && depth == 0 {
				return i + 1
			}
		case "nl":
			if sawColon && !started {
				if i+1 < len(toks) && toks[i+1].Kind == "indent" {
					continue
				}
				return i + 1
			}
		}
	}
	return len(toks)
}

func scorePython(toks []Token) (int, int) {
	cog, nest, maxN := 0, 0, 0
	pending := 0
	prevLogic := ""
	logicLive := false
	for _, t := range toks {
		if t.Kind == "indent" {
			nest += pending
			pending = 0
			if nest > maxN {
				maxN = nest
			}
			continue
		}
		if t.Kind == "dedent" {
			if nest > 0 {
				nest--
			}
			logicLive = false
			prevLogic = ""
			continue
		}
		if t.Kind == "nl" || (t.Kind == "punct" && t.Text == ":") {
			logicLive = false
			prevLogic = ""
			continue
		}
		if t.Kind == "kw" && pythonControl(t.Text) {
			cog += 1 + nest
			if t.Text != "else" && t.Text != "elif" {
				pending++
			}
			logicLive = false
			prevLogic = ""
			continue
		}
		if t.Kind == "kw" && (t.Text == "and" || t.Text == "or") {
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

func pythonControl(s string) bool {
	switch s {
	case "if", "elif", "else", "for", "while", "except", "with", "match", "case":
		return true
	default:
		return false
	}
}
