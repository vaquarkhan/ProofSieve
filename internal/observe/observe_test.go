package observe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vaquarkhan/ProofSieve/internal/report"
)

func TestExport(t *testing.T) {
	if err := Export(context.Background(), "", report.Report{}); err != nil {
		t.Fatal(err)
	}
	risk, ratio := 74, 0.5
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		got = string(buf)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	rep := report.Report{SHA: "abc", Files: 2, DiffLOC: 7, RiskScore: &risk, MutationTier1: &ratio, FindingsRaw: 3, FindingsKept: 1}
	if err := Export(context.Background(), srv.URL, rep); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "proofsieve") || !strings.Contains(line(rep), "risk=74") || !strings.Contains(line(rep), "mutation_tier1=0.5000") {
		t.Fatal(got)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()
	if err := Export(context.Background(), bad.URL, rep); err == nil {
		t.Fatal("status")
	}
	orig := post
	post = func(context.Context, string, []byte) (int, error) { return 0, errors.New("net") }
	t.Cleanup(func() { post = orig })
	if err := Export(context.Background(), "http://example.invalid", rep); err == nil {
		t.Fatal("net")
	}
	origMarshal := marshal
	marshal = func(any) ([]byte, error) { return nil, errors.New("json") }
	t.Cleanup(func() { marshal = origMarshal })
	if err := Export(context.Background(), srv.URL, rep); err == nil {
		t.Fatal("marshal")
	}
}

func TestBadURL(t *testing.T) {
	if err := Export(context.Background(), "http://[::1", report.Report{}); err == nil {
		t.Fatal("url")
	}
	if err := Export(context.Background(), "http://127.0.0.1:1", report.Report{}); err == nil {
		t.Fatal("dial")
	}
	empty := line(report.Report{})
	if !strings.Contains(empty, "risk=") {
		t.Fatal(empty)
	}
}
