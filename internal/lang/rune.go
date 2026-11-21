package lang

import "unicode/utf8"

func runeFrom(b []byte) (rune, int) {
	return utf8.DecodeRune(b)
}
