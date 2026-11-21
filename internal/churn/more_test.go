package churn

import "testing"

func TestClassEdges(t *testing.T) {
	if Class("python", "return x\n", "x return\n") != "functional" {
		t.Fatal("kind")
	}
	if Class("python", "a + a\n", "b + c\n") != "functional" {
		t.Fatal("conflict")
	}
	if Lines("abc", "de") < 2 {
		t.Fatal("no newline")
	}
}
