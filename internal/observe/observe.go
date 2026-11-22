// Package observe exports one check result to an OpenTelemetry HTTP endpoint.
package observe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/vaquarkhan/ProofSieve/internal/report"
)

var (
	post = func(ctx context.Context, endpoint string, body []byte) (int, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, err
		}
		defer res.Body.Close()
		_, _ = io.Copy(io.Discard, res.Body)
		return res.StatusCode, nil
	}
	marshal = json.Marshal
)

// Export posts one OTLP JSON log record. An empty endpoint does nothing.
func Export(ctx context.Context, endpoint string, rep report.Report) error {
	if endpoint == "" {
		return nil
	}
	payload := map[string]any{
		"resourceLogs": []any{map[string]any{
			"resource": map[string]any{"attributes": []any{
				attr("service.name", "proofsieve"),
			}},
			"scopeLogs": []any{map[string]any{
				"logRecords": []any{map[string]any{
					"timeUnixNano": strconv.FormatInt(time.Now().UnixNano(), 10),
					"body":         map[string]any{"stringValue": line(rep)},
					"attributes": []any{
						attr("proofsieve.sha", rep.SHA),
						attr("proofsieve.files", strconv.Itoa(rep.Files)),
						attr("proofsieve.diff_loc", strconv.Itoa(rep.DiffLOC)),
					},
				}},
			}},
		}},
	}
	raw, err := marshal(payload)
	if err != nil {
		return err
	}
	code, err := post(ctx, endpoint, raw)
	if err != nil {
		return err
	}
	if code < 200 || code >= 300 {
		return fmt.Errorf("otel endpoint returned %d", code)
	}
	return nil
}

func line(rep report.Report) string {
	risk := ""
	if rep.RiskScore != nil {
		risk = strconv.Itoa(*rep.RiskScore)
	}
	mut := ""
	if rep.MutationTier1 != nil {
		mut = strconv.FormatFloat(*rep.MutationTier1, 'f', 4, 64)
	}
	return fmt.Sprintf("sha=%s files=%d diff_loc=%d risk=%s mutation_tier1=%s findings_raw=%d findings_kept=%d",
		rep.SHA, rep.Files, rep.DiffLOC, risk, mut, rep.FindingsRaw, rep.FindingsKept)
}

func attr(key, value string) map[string]any {
	return map[string]any{"key": key, "value": map[string]any{"stringValue": value}}
}
