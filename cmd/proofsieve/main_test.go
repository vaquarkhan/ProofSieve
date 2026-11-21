package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMainVersion(t *testing.T) {
	origExit := exit
	origArgs := os.Args
	origOut := os.Stdout
	defer func() {
		exit = origExit
		os.Args = origArgs
		os.Stdout = origOut
	}()
	var code int
	exit = func(c int) { code = c }
	os.Args = []string{"proofsieve", "version"}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	main()
	_ = w.Close()
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || !strings.Contains(string(body), "proofsieve 1.0.0") {
		t.Fatalf("%d %s", code, body)
	}
}

func TestRunHelp(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"help"}, strings.NewReader(""), &out, &err); code != 0 || !strings.Contains(out.String(), "proofsieve lock") {
		t.Fatalf("%d %s", code, out.String())
	}
}
