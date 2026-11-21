package churn

import "testing"

func TestClass(t *testing.T) {
	if Class("python", "x = 1\n", "x = 1\n") != "formatting" {
		t.Fatal("same")
	}
	if Class("python", "x = 1\n", "x  =  1\n") != "formatting" {
		t.Fatal("space")
	}
	if Class("python", "total = 1\n", "amount = 1\n") != "rename" {
		t.Fatal("rename")
	}
	if Class("python", "total = 1\n", "total = 2\n") != "functional" {
		t.Fatal("number")
	}
	if Class("python", "total = 1\n", "amount = 1\nvalue = 1\n") != "functional" {
		t.Fatal("len")
	}
	if Class("python", "return 'unterminated\n", "return 1\n") != "functional" {
		t.Fatal("lex")
	}
	if Class("python", "a = b\n", "a = c\nb = c\n") != "functional" {
		t.Fatal("conflict length")
	}
	if Lines("a\n", "b\nc\n") != 3 {
		t.Fatal(Lines("a\n", "b\nc\n"))
	}
	if Lines("", "no-nl") != 1 {
		t.Fatal("nonewline")
	}
}

func TestRenameConflict(t *testing.T) {
	if Class("python", "a = b\n", "c = c\n") != "functional" {
		t.Fatal("bijection")
	}
}
