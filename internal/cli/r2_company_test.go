package cli

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestR2CompanySelectionRequiresServedFile(t *testing.T) {
	for _, rows := range []string{`[{"name":"Acme","database_name":"acme","id":"acme"}]`, `[{"file":"acme.db","name":"Acme","database_name":"acme","id":"acme"}]`} {
		t.Run(rows, func(t *testing.T) {
			writes := 0
			reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/api/companies" {
					_, _ = w.Write([]byte(rows))
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
			for _, company := range []string{"Acme", "acme"} {
				code, out, _ := invoke(t, "", "api", "POST", "/api/customers", "--company", company, "--data", "{}")
				if code != 2 || writes != 0 {
					t.Errorf("%s: %d %s", company, code, out)
				}
				if !strings.Contains(rows, `"file"`) && !strings.Contains(out, "this server does not support company selection") {
					t.Error(out)
				}
			}
			if strings.Contains(rows, `"file"`) {
				code, out, _ := invoke(t, "", "api", "POST", "/api/customers", "--company", "acme.db", "--data", "{}")
				if code != 0 || writes != 1 {
					t.Fatal(code, out, writes)
				}
			}
		})
	}
}
func TestR2ReadsRefuseUnknownCompanies(t *testing.T) {
	reads := 0
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/companies" {
			_, _ = w.Write([]byte(`[{"file":"acme.db","name":"Acme"}]`))
			return true
		}
		if r.URL.Path == "/api/customers" || strings.HasPrefix(r.URL.Path, "/api/reports/") || strings.HasSuffix(r.URL.Path, "/pdf") {
			reads++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return true
		}
		return false
	})
	for _, args := range [][]string{{"customers", "list"}, {"reports", "run", "profit-loss"}, {"invoices", "pdf", "1", "-o", filepath.Join(t.TempDir(), "test.pdf")}, {"auth", "status"}} {
		code, out, _ := invoke(t, "", append(args, "--company", "bogus")...)
		if code != 2 || reads != 0 || !strings.Contains(out, "acme.db") {
			t.Errorf("%v: %d %s reads=%d", args, code, out, reads)
		}
	}
}

func TestR2UnverifiableReadWarnsAndContinues(t *testing.T) {
	reads := 0
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/companies" {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"detail":"probe unavailable"}`))
			return true
		}
		if r.URL.Path == "/api/customers" {
			reads++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return true
		}
		return false
	})
	code, out, stderr := invoke(t, "", "customers", "list", "--company", "acme")
	if code != 0 || reads != 1 || !strings.Contains(stderr, "could not verify company acme: probe unavailable") {
		t.Fatal(code, out, stderr, reads)
	}
}
