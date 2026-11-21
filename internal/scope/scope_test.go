package scope

import (
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
)

func TestDecide(t *testing.T) {
	lock := lockfile.Lock{Blast: lockfile.Blast{
		MaxFiles: 2, MaxLoc: 10,
		Allowed: []string{"billing/**"},
		Denied:  []string{"billing/secret/**"},
	}}
	if !Decide(lock, "billing/a.py", 0, 1, true).Allow {
		t.Fatal("allow")
	}
	if Decide(lock, ".proofsieve/policy.yaml", 0, 1, true).Rule != "policy-path" {
		t.Fatal("policy")
	}
	if Decide(lock, "billing/secret/a.py", 0, 1, true).Rule != "denied-path" {
		t.Fatal("deny")
	}
	if Decide(lock, "auth/a.py", 0, 1, true).Rule != "outside-scope" {
		t.Fatal("outside")
	}
	if Decide(lock, "billing/b.py", 2, 1, true).Rule != "max-files" {
		t.Fatal("files")
	}
	if Decide(lock, "billing/a.py", 0, 11, false).Rule != "max-loc" {
		t.Fatal("loc")
	}
	lock.Blast.Allowed = []string{"../x"}
	if Decide(lock, "billing/a.py", 0, 1, true).Allow {
		t.Fatal("bad glob")
	}
}
