package attest

import "testing"

func TestParse(t *testing.T) {
	text := "/proofsieve attest\nsha: abc\nspec: specs/a.md#REQ-1\ninvariant: half-up stays stable\nfailure: rollback the pricing flag\ntest: test_half_up\nbranch: price\ncall: eval\n"
	a, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	id, err := SpecID(a.Spec)
	if err != nil || id != "REQ-1" {
		t.Fatal(id, err)
	}
	if !HoldsSpec("The system meets REQ-1.", id) {
		t.Fatal("spec")
	}
	if a.Branch != "price" || a.Call != "eval" {
		t.Fatal(a.Branch, a.Call)
	}
	if !HoldsTest("def test_half_up():\n    assert True\n", a.Test) {
		t.Fatal("test")
	}
	if HoldsTest("def test_half_up():\n    pytest.mark.skip\n", "test_half_up") {
		t.Fatal("skip")
	}
	if HoldsTest("def test_half_up_extra():\n    pass\n", "test_half_up") {
		t.Fatal("prefix")
	}
	if AuthorOK("") || AuthorOK("proofsieve[bot]") || !AuthorOK("vaquarkhan") {
		t.Fatal("author")
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse("hello"); err == nil {
		t.Fatal("header")
	}
	if _, err := Parse("/proofsieve attest\nsha abc"); err == nil {
		t.Fatal("colon")
	}
	if _, err := Parse("/proofsieve attest\nsha: a\nnope: b\n"); err == nil {
		t.Fatal("unknown")
	}
	if _, err := Parse("/proofsieve attest\nsha: a\nspec: REQ-1\ninvariant: short\nfailure: long enough text\ntest: t\n"); err == nil {
		t.Fatal("short")
	}
	if _, err := SpecID("specs/a.md"); err == nil {
		t.Fatal("no id")
	}
	if _, err := SpecID(""); err == nil {
		t.Fatal("empty")
	}
	if HoldsSpec("x", "") || HoldsTest("", "t") {
		t.Fatal("empty holds")
	}
}
