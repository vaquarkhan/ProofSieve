package globset

import "testing"

func TestCompileRejects(t *testing.T) {
	if _, err := Compile([]string{" "}); err == nil {
		t.Fatal("empty")
	}
	if _, err := Compile([]string{"../secret"}); err == nil {
		t.Fatal("dotdot")
	}
}

func TestMatch(t *testing.T) {
	set, err := Compile([]string{
		"services/billing/src/**",
		"**/package-lock.json",
		"*.py",
		"billing/oracle/*",
		".proofsieve/**",
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		want bool
	}{
		{"services/billing/src/a.py", true},
		{"services/billing/src/x/y.go", true},
		{"services/billing/src", true},
		{"services/other/a.py", false},
		{"a/package-lock.json", true},
		{"package-lock.json", true},
		{"root.py", true},
		{"dir/root.py", false},
		{"billing/oracle/test_rounding.py", true},
		{"billing/oracle/nested/x.py", false},
		{".proofsieve/policy.yaml", true},
		{".proofsieve/run/session.json", true},
		{`.proofsieve\policy.yaml`, true},
		{"nope", false},
	}
	for _, tc := range cases {
		if got := set.Match(tc.path); got != tc.want {
			t.Errorf("Match(%q)=%v want %v", tc.path, got, tc.want)
		}
	}
	if len(set.Patterns()) != 5 {
		t.Fatalf("patterns %d", len(set.Patterns()))
	}
	empty, err := Compile(nil)
	if err != nil || empty.Match("a") {
		t.Fatalf("empty set matched: %v", err)
	}
}

func TestStarQuestion(t *testing.T) {
	set, err := Compile([]string{"src/a?c.txt", "src/*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !set.Match("src/abc.txt") || set.Match("src/ac.txt") || !set.Match("src/main.go") {
		t.Fatalf("wildcard match failed")
	}
}
