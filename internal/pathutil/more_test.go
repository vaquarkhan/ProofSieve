package pathutil

import (
	"errors"
	"testing"
)

func TestRelVolume(t *testing.T) {
	_, _ = Rel(`C:\repo`, `D:\other\file.go`)
	_, _ = Rel(`C:\repo`, `\\server\share\file.go`)
}

func TestRelFuncError(t *testing.T) {
	orig := filepathRel
	filepathRel = func(string, string) (string, error) { return "", errors.New("rel") }
	t.Cleanup(func() { filepathRel = orig })
	if _, err := Rel(t.TempDir(), "a.go"); err == nil {
		t.Fatal("rel")
	}
}
