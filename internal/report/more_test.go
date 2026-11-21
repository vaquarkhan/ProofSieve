package report

import (
	"errors"
	"testing"
)

func TestEmptyAndMarshal(t *testing.T) {
	var r Report
	if r.JSON() == "" || r.SARIF() == "" || r.Markdown() == "" {
		t.Fatal("empty")
	}
	orig := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("marshal") }
	t.Cleanup(func() { marshalIndent = orig })
	if r.JSON() != "{\"error\":\"marshal\"}\n" || r.SARIF() != "{}\n" {
		t.Fatal(r.JSON(), r.SARIF())
	}
}
