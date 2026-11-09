package cli

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestBusinessRefusalGolden(t *testing.T) {
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != "POST" {
			return false
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"detail":"Bill payment already voided"}`))
		return true
	})
	code, out, _ := invoke(t, "", "api", "POST", "/api/bill-payments/1/void")
	want, err := os.ReadFile("../../testdata/golden/business-refusal.json")
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 || out != string(want) {
		t.Fatal(code, out)
	}
}

func TestPaymentRetryLimitationsDocumented(t *testing.T) {
	isolate(t)
	for _, path := range []string{"../../docs/adr/0001-caller-supplied-payment-key.md", "../../skills/slowbooks-cli/SKILL.md"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range []string{"POST /api/batch-payments", "/replace", "Idempotency-Key", "never re-run"} {
			if !strings.Contains(string(raw), term) {
				t.Errorf("%s missing %s", path, term)
			}
		}
	}
}

func TestDeclaredJSONWriteRejectsPage(t *testing.T) {
	hits := 0
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != "POST" {
			return false
		}
		hits++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>Sign in</html>"))
		return true
	})
	for _, args := range [][]string{{"api", "POST", "/api/customers", "--data", "{}"}, {"api", "POST", "/api/{resource}", "--param", "resource=customers", "--data", "{}"}, {"invoices", "void", "1"}, {"api", "POST", "/api/payments", "--key", "test", "--data", "{}"}} {
		before := hits
		code, out, _ := invoke(t, "", args...)
		if code != 1 || !strings.Contains(out, `"retryable":false`) || !strings.Contains(out, "the server answered with a page, not JSON; the write may have happened") || !strings.Contains(out, "check the books") || hits != before+1 {
			t.Errorf("%v: %d %s hits=%d", args, code, out, hits-before)
		}
	}
}

func TestDeclaredJSONWriteRejectsMislabelledPage(t *testing.T) {
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != "POST" {
			return false
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("<html>Gateway sign in</html>"))
		return true
	})
	code, out, _ := invoke(t, "", "api", "POST", "/api/customers", "--data", "{}")
	if code != 1 || !strings.Contains(out, "the write may have happened") || !strings.Contains(out, "check the books") || !strings.Contains(out, `"retryable":false`) {
		t.Fatal(code, out)
	}
}
