package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/testdata"
)

func TestR2UnkeyedWritesNeverRecommendReplay(t *testing.T) {
	for _, drop := range []bool{false, true} {
		t.Run(map[bool]string{false: "gateway", true: "lost response"}[drop], func(t *testing.T) {
			reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
				if r.Method != "POST" {
					return false
				}
				if drop {
					c, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return true
					}
					_ = c.Close()
				} else {
					w.WriteHeader(503)
				}
				return true
			})
			for _, path := range []string{"/api/customers", "/api/invoices", "/api/deposits", "/api/journal", "/api/payments/5/void", "/api/batch-payments", "/api/payments/5/replace"} {
				code, out, stderr := invoke(t, "", "api", "POST", path, "--key", "k1", "--data", "{}")
				if code != 8 || !strings.Contains(out, `"retryable":false`) || !strings.Contains(out, "check the books") || strings.Contains(out, "rerun with the same") || !strings.Contains(stderr, "server ignores --key") {
					t.Errorf("%s: %d %s %s", path, code, out, stderr)
				}
			}
			code, out, stderr := invoke(t, "", "api", "POST", "/api/payments", "--key", "k1", "--data", "{}")
			if code != 8 || !strings.Contains(out, `"retryable":true`) || !strings.Contains(out, "rerun with the same --key") || strings.Contains(stderr, "server ignores") {
				t.Fatal(code, out, stderr)
			}
		})
	}
}

func TestR2CatalogKeyHeaderEnablesReplayAdvice(t *testing.T) {
	raw, err := testdata.Files.ReadFile("openapi-2.31.10.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["paths"].(map[string]any)["/api/custom-dedupe"] = map[string]any{"post": map[string]any{"parameters": []any{map[string]any{"in": "header", "name": "Idempotency-Key"}}}}
	writes := 0
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/openapi.json" {
			_ = json.NewEncoder(w).Encode(doc)
			return true
		}
		if r.Method == "POST" {
			writes++
			w.WriteHeader(503)
			return true
		}
		return false
	})
	code, out, stderr := invoke(t, "", "ops", "call", "POST", "/api/custom-dedupe", "--key", "k1", "--data", "{}")
	if code != 8 || writes != 1 || !strings.Contains(out, `"retryable":true`) || !strings.Contains(out, "rerun with the same --key") || strings.Contains(stderr, "server ignores") {
		t.Fatal(code, out, stderr)
	}
}
