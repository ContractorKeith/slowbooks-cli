package cli

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanyProbeFailureWarnsAndAllowsLoginAndWrites(t *testing.T) {
	for _, status := range []int{403, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			writes := 0
			reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/api/companies" {
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"detail":"company list unavailable"}`))
					return true
				}
				if r.Method == "POST" {
					writes++
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"id":3}`))
					return true
				}
				return false
			})
			for _, args := range [][]string{{"api", "POST", "/api/customers", "--data", "{}"}, {"invoices", "void", "1"}} {
				before := writes
				code, out, stderr := invoke(t, "", append(args, "--company", "acme")...)
				if code != 0 || writes != before+1 || !strings.Contains(stderr, "could not verify company acme: company list unavailable") {
					t.Errorf("%v: %d %s %s", args, code, out, stderr)
				}
			}
		})
	}
	_, server := setupAuth(t)
	code, out, stderr := invoke(t, "sbp_no_companies", "auth", "login", "--url", server, "--company", "acme")
	if code != 0 || !strings.Contains(stderr, "could not verify company acme:") {
		t.Fatal(code, out, stderr)
	}
	for _, args := range [][]string{{"api", "POST", "/api/customers", "--data", "{}"}, {"invoices", "void", "1"}} {
		code, out, stderr = invoke(t, "", args...)
		if code != 0 || !strings.Contains(stderr, "could not verify company acme:") {
			t.Fatal(code, out, stderr)
		}
	}

}

func TestNamedSuccessHintsUseExactRecordID(t *testing.T) {
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/invoices/3/pdf" {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF fixture"))
			return true
		}
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":3}`))
			return true
		}
		return false
	})
	for _, tc := range []struct {
		args []string
		next string
	}{
		{[]string{"invoices", "pdf", "3", "-o", filepath.Join(t.TempDir(), "invoice.pdf")}, "sbp invoices show 3"},
		{[]string{"invoices", "create", "--customer", "1", "--date", "2026-11-09"}, "sbp invoices show 3"},
		{[]string{"payments", "record", "--customer", "1", "--date", "2026-11-09", "--amount", "5", "--key", "test"}, "sbp payments show 3"},
	} {
		code, out, _ := invoke(t, "", tc.args...)
		var response struct{ Next []string }
		err := json.Unmarshal([]byte(out), &response)
		if code != 0 || err != nil || len(response.Next) != 1 || response.Next[0] != tc.next {
			t.Errorf("%v: %d %s", tc.args, code, out)
		}
	}
}

func TestWriteJSONWithoutContentType(t *testing.T) {
	for _, header := range []struct {
		name   string
		values []string
	}{{"missing", nil}, {"blank", []string{""}}, {"whitespace", []string{" "}}} {
		t.Run(header.name, func(t *testing.T) {
			reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
				if r.Method != "POST" {
					return false
				}
				w.Header()["Content-Type"] = header.values
				_, _ = w.Write([]byte(`{"id":9007199254740993}`))
				return true
			})
			code, out, _ := invoke(t, "", "api", "POST", "/api/customers", "--data", "{}")
			if code != 0 || !strings.Contains(out, `"id":9007199254740993`) {
				t.Fatal(code, out)
			}
		})
	}
}

func TestMissingStatusEnumWarnsAndPassesThrough(t *testing.T) {
	fake := reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/openapi.json" {
			return false
		}
		_, _ = w.Write([]byte(`{"openapi":"3.1.0","paths":{"/api/invoices":{"get":{"parameters":[{"in":"query","name":"status","schema":{"type":"string"}}]}}}}`))
		return true
	})
	code, out, stderr := invoke(t, "", "invoices", "list", "--status", "future")
	if code != 0 || !strings.Contains(stderr, "catalog does not declare status values") {
		t.Fatal(code, out, stderr)
	}
	reqs := fake.Requests()
	if !strings.Contains(reqs[len(reqs)-1].URI, "status=future") {
		t.Fatal(reqs)
	}
}

func TestFortyMiBResponseSupportsJQIDProjection(t *testing.T) {
	padding := strings.Repeat("x", 40<<20)
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != "GET" || r.URL.Path != "/api/customers" {
			return false
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": json.Number("9007199254740993"), "padding": padding}})
		return true
	})
	code, out, _ := invoke(t, "", "customers", "list", "--all", "--jq", ".data[].id")
	if code != 0 || out != "9007199254740993\n" {
		t.Fatal(code, out)
	}
}

func TestGenericStatusParameterBypassesNamedValidation(t *testing.T) {
	fake := reviewServer(t, nil)
	code, out, _ := invoke(t, "", "api", "GET", "/api/invoices", "--param", "status=unknown")
	reqs := fake.Requests()
	if code != 0 || !strings.Contains(reqs[len(reqs)-1].URI, "status=unknown") {
		t.Fatal(code, out)
	}
}
func TestSingularReadNouns(t *testing.T) {
	isolate(t)
	for plural, want := range map[string]string{"invoices": "invoice", "expenses": "expense", "bill-payments": "bill-payment", "journal": "journal entry"} {
		if got := singular(plural); got != want {
			t.Errorf("%s: %s", plural, got)
		}
	}
}
