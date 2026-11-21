// Package integrity detects tests that were deleted, skipped, or stripped of assertions.
package integrity

import (
	"strings"

	"github.com/vaquarkhan/ProofSieve/internal/lang"
	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
	"github.com/vaquarkhan/ProofSieve/internal/report"
)

// Check compares a test file before and after a change.
func Check(path, base, head string, lock lockfile.Lock) []report.Finding {
	if !lang.IsTest(path) || lock.Tests.AllowMutation {
		return nil
	}
	var out []report.Finding
	if head == "" && base != "" && !lock.Tests.AllowDelete {
		out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "test-deleted", Path: path, Message: "a test file was deleted"})
		return out
	}
	if !lock.Tests.AllowSkip {
		for i, line := range addedLines(base, head) {
			if skipLine(line) {
				out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "test-skipped", Path: path, Line: i, Message: "a skip annotation was added to a test"})
				break
			}
		}
	}
	if assertions(base) > assertions(head) {
		out = append(out, report.Finding{Gate: "scope", Severity: "error", Rule: "assertion-removed", Path: path, Message: "an assertion was removed from a test"})
	}
	return out
}

func addedLines(base, head string) map[int]string {
	baseSet := map[string]int{}
	for _, line := range strings.Split(base, "\n") {
		baseSet[strings.TrimSpace(line)]++
	}
	out := map[int]string{}
	for i, line := range strings.Split(head, "\n") {
		trim := strings.TrimSpace(line)
		if baseSet[trim] > 0 {
			baseSet[trim]--
			continue
		}
		out[i+1] = trim
	}
	return out
}

func skipLine(line string) bool {
	needles := []string{
		"pytest.mark.skip", "unittest.skip", "@Disabled", "@Ignore",
		"it.skip", "describe.skip", "test.skip", "xit(", "xdescribe(",
		".skip(", "t.Skip(",
	}
	for _, n := range needles {
		if strings.Contains(line, n) {
			return true
		}
	}
	return false
}

func assertions(src string) int {
	n := 0
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "assert ") || strings.HasPrefix(trim, "assert(") ||
			strings.Contains(trim, "expect(") || strings.Contains(trim, "require(") ||
			strings.Contains(trim, "t.Fatal") || strings.Contains(trim, "t.Error") ||
			strings.Contains(trim, "self.assert") {
			n++
		}
	}
	return n
}
