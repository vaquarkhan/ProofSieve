package diffparse

import "testing"

func TestParseEdges(t *testing.T) {
	files, err := Parse("diff --git a/a.py b/a.py\n--- file.py\n+++ file.py\n@@ -1 +1 @@\n-a\n+b\n")
	if err != nil || len(files) != 1 || files[0].Path() != "file.py" {
		t.Fatal(err, files)
	}
	if _, err = Parse("diff --git a/a b/a\n@@ nope\n"); err == nil {
		t.Fatal("hunk")
	}
	if _, err = parseHunk("nope"); err == nil {
		t.Fatal("prefix")
	}
	if _, err = parseHunk("@@ -1 @@"); err == nil {
		t.Fatal("parts")
	}
	if _, err = parseHunk("@@ -x +1 @@"); err == nil {
		t.Fatal("span")
	}
	if _, err = parseHunk("@@ -1 +x @@"); err == nil {
		t.Fatal("new span")
	}
	if _, err = parseHunk("@@ -1,y +1 @@"); err == nil {
		t.Fatal("old count")
	}
	if _, _, err = span("-0"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = span("-1,x"); err == nil {
		t.Fatal("count")
	}
	if errHunk("@@").Error() == "" {
		t.Fatal("error string")
	}
	if len(NewLines(File{Hunks: []Hunk{{NewText: ""}}})) != 0 {
		t.Fatal("empty hunk")
	}
}
