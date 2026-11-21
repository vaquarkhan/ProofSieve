package attest

import "testing"

func TestParseAndIdentEdges(t *testing.T) {
	if _, err := Parse("sha: abc\n"); err == nil {
		t.Fatal("header")
	}
	if _, err := Parse("/proofsieve attest\nsha: abc\n"); err == nil {
		t.Fatal("missing")
	}
	if HoldsTest("def test_auth():\n    pass\n", "") {
		t.Fatal("empty name")
	}
	if HoldsTest("def xtest_auth():\n    pass\n", "test_auth") {
		t.Fatal("prefix")
	}
	if !HoldsTest("def test_auth():\n    pass\n", "test_auth") {
		t.Fatal("exact")
	}
	if _, err := SpecID("specs/a.md#"); err == nil {
		t.Fatal("empty hash")
	}
}
