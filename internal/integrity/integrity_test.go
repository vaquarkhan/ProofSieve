package integrity

import (
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
)

func TestCheck(t *testing.T) {
	lock := lockfile.Lock{}
	base := "def test_a():\n    assert 1 == 1\n"
	head := "def test_a():\n    assert 1 == 1\n    pytest.mark.skip\n"
	got := Check("tests/test_a.py", base, head, lock)
	if len(got) != 1 || got[0].Rule != "test-skipped" {
		t.Fatalf("%+v", got)
	}
	weak := "def test_a():\n    return\n"
	got = Check("tests/test_a.py", base, weak, lock)
	if len(got) != 1 || got[0].Rule != "assertion-removed" {
		t.Fatalf("%+v", got)
	}
	got = Check("tests/test_a.py", base, "", lock)
	if len(got) != 1 || got[0].Rule != "test-deleted" {
		t.Fatalf("%+v", got)
	}
	lock.Tests.AllowMutation = true
	if Check("tests/test_a.py", base, "", lock) != nil {
		t.Fatal("mutation allowed")
	}
	if Check("billing/rounding.py", base, "", lockfile.Lock{}) != nil {
		t.Fatal("prod file")
	}
	lock = lockfile.Lock{}
	lock.Tests.AllowDelete = true
	lock.Tests.AllowSkip = true
	got = Check("pkg/a_test.go", "t.Fatal(1)\n", "t.Fatal(1)\n", lock)
	if len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}
