package scope

import (
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
)

func TestBadGlob(t *testing.T) {
	d := Decide(lockfile.Lock{Blast: lockfile.Blast{Denied: []string{".."}, Allowed: []string{"**"}}}, "a.py", 0, 0, false)
	if d.Allow || d.Rule != "lock" {
		t.Fatal(d)
	}
	d = Decide(lockfile.Lock{Blast: lockfile.Blast{Allowed: []string{""}}}, "a.py", 0, 0, false)
	if d.Allow {
		t.Fatal(d)
	}
}
