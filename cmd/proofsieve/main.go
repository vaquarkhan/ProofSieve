package main

import (
	"io"
	"os"

	"github.com/vaquarkhan/ProofSieve/internal/cli"
)

var exit = os.Exit

func main() {
	exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, stdout, stderr io.Writer) int {
	return cli.Execute(args, in, stdout, stderr)
}
