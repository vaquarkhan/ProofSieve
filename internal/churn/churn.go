// Package churn classifies a hunk as formatting, rename, or functional.
package churn

import (
	"strings"

	"github.com/vaquarkhan/ProofSieve/internal/lang"
)

// Class returns formatting, rename, or functional.
func Class(language, oldText, newText string) string {
	if strings.TrimSpace(oldText) == strings.TrimSpace(newText) {
		return "formatting"
	}
	oldTok, err1 := lang.Tokens(language, oldText)
	newTok, err2 := lang.Tokens(language, newText)
	if err1 != nil || err2 != nil {
		return "functional"
	}
	if len(oldTok) != len(newTok) {
		return "functional"
	}
	rename := map[string]string{}
	used := map[string]string{}
	changed := false
	for i := range oldTok {
		if oldTok[i].Kind != newTok[i].Kind {
			return "functional"
		}
		if oldTok[i].Text == newTok[i].Text {
			continue
		}
		if oldTok[i].Kind != "ident" {
			return "functional"
		}
		changed = true
		if prev, ok := rename[oldTok[i].Text]; ok && prev != newTok[i].Text {
			return "functional"
		}
		if prev, ok := used[newTok[i].Text]; ok && prev != oldTok[i].Text {
			return "functional"
		}
		rename[oldTok[i].Text] = newTok[i].Text
		used[newTok[i].Text] = oldTok[i].Text
	}
	if changed {
		return "rename"
	}
	return "formatting"
}

// Lines counts added and deleted lines in a hunk body.
func Lines(oldText, newText string) int {
	return count(oldText) + count(newText)
}

func count(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
