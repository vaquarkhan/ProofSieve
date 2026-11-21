package version

import "testing"

func TestString(t *testing.T) {
	if String() != "proofsieve 1.0.0" {
		t.Fatal(String())
	}
}
