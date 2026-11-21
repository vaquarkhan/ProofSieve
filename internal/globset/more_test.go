package globset

import "testing"

func TestMatchEdges(t *testing.T) {
	set, err := Compile([]string{"a/b", "/", "x?"})
	if err != nil {
		t.Fatal(err)
	}
	if !set.Match(`a//b`) {
		t.Fatal("slash")
	}
	_ = set.Match("/")
	if set.Match("x") {
		t.Fatal("question")
	}
	if matchParts([]string{"**", "a"}, nil) {
		t.Fatal("double star empty")
	}
}
