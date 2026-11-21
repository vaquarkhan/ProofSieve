package diffparse

import "testing"

func TestParse(t *testing.T) {
	diff := "" +
		"diff --git a/billing/rounding.py b/billing/rounding.py\n" +
		"--- a/billing/rounding.py\n" +
		"+++ b/billing/rounding.py\n" +
		"@@ -2,1 +2,2 @@\n" +
		"-    return 1\n" +
		"+    if x:\n" +
		"+        return 2\n" +
		"\\ No newline at end of file\n" +
		"diff --git a/new.py b/new.py\n" +
		"--- /dev/null\n" +
		"+++ b/new.py\n" +
		"@@ -0,0 +1 @@\n" +
		"+print(1)\n" +
		"diff --git a/old.py b/old.py\n" +
		"--- a/old.py\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n" +
		"-print(1)\n"
	files, err := Parse(diff)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("%d", len(files))
	}
	if files[0].Added != 2 || files[0].Removed != 1 || files[0].Path() != "billing/rounding.py" {
		t.Fatalf("%+v", files[0])
	}
	lines := NewLines(files[0])
	if !lines[2] || !lines[3] {
		t.Fatalf("%v", lines)
	}
	if !files[1].AddedFile() || files[1].Path() != "new.py" {
		t.Fatal(files[1])
	}
	if !files[2].Deleted() || files[2].Path() != "old.py" || len(NewLines(files[2])) != 0 {
		t.Fatal(files[2])
	}
	if _, err := Parse("@@ not a hunk\n"); err == nil {
		t.Fatal("bad hunk")
	}
	if _, err := Parse("@@ -x +1 @@\n"); err == nil {
		t.Fatal("bad span")
	}
}
