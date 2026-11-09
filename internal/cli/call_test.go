package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericCallsAndSafety(t *testing.T) {
	fake, server := setupAuth(t)
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	if code != 0 {
		t.Fatal(out)
	}
	code, out, _ = invoke(t, "", "ops", "find", "customer")
	if code != 0 || !strings.Contains(out, `/api/customers`) {
		t.Fatal(code, out)
	}
	before := len(fake.Requests())
	code, out, _ = invoke(t, "", "api", "POST", "/api/payments", "--key", "check-1", "--data", `{"amount":1}`, "--dry-run")
	if code != 0 || len(fake.Requests()) != before || strings.Contains(out, "sbp_bookkeeper") || !strings.Contains(out, "redacted") {
		t.Fatal(code, out, len(fake.Requests()), before)
	}
	for _, name := range [][]string{{"api"}, {"ops", "call"}} {
		args := append(append([]string{}, name...), "POST", "/api/payments", "--data", `{}`)
		code, out, _ = invoke(t, "", args...)
		if code != 2 || !strings.Contains(out, "--key") {
			t.Fatal(code, out)
		}
		args = append(append([]string{}, name...), "POST", "/api/payments", "--key", "check-2", "--data", `{"amount":1}`)
		code, out, _ = invoke(t, "", args...)
		if code != 0 {
			t.Fatal(code, out)
		}
		reqs := fake.Requests()
		last := reqs[len(reqs)-1]
		if last.Header.Get("Idempotency-Key") != "check-2" || string(last.Body) != `{"amount":1}` {
			t.Fatal(last)
		}
	}
	cases := [][]string{
		{"ops", "call", "GET", "/api/not-in-catalog"},
		{"api", "GET", "/api/invoices/{invoice_id}/pdf", "--param", "invoice_id=1"},
		{"api", "POST", "/api/payments/123/discount"},
		{"api", "POST", "/api/{resource}", "--param", "resource=payments"},
		{"api", "GET", "/api/foo/{format}", "--param", "format=pdf"},
		{"api", "GET", "/api/customers/{customer_id}"},
		{"api", "POST", "/api/customers", "--data", "bad"},
		{"api", "GET", "/api/customers", "--param", "bad"},
	}
	for _, args := range cases {
		code, out, _ = invoke(t, "", args...)
		if code != 2 {
			t.Fatal(args, code, out)
		}
	}
	code, _, stderr := invoke(t, "", "api", "GET", "/api/not-in-catalog")
	if code != 5 || !strings.Contains(stderr, "warning") {
		t.Fatal(code, stderr)
	}
}
func TestParamsAndDataSources(t *testing.T) {
	fake, server := setupAuth(t)
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	if code != 0 {
		t.Fatal(out)
	}
	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, []byte(`{"name":"File customer"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{"@" + file, "-", `{"name":"Inline customer"}`} {
		code, out, _ = invoke(t, `{"name":"stdin customer"}`, "ops", "call", "POST", "/api/customers", "--data", data)
		if code != 0 {
			t.Fatal(code, out)
		}
	}
	code, out, _ = invoke(t, "", "api", "GET", "/api/customers/{customer_id}", "--param", "customer_id=42", "--param", "q=a&b", "--param", "q=two", "--dry-run")
	if code != 0 || !strings.Contains(out, "/api/customers/42?q=a%26b\\u0026q=two") {
		t.Fatal(code, out)
	}
	reqs := fake.Requests()
	count := 0
	for _, r := range reqs {
		if r.Method == "POST" && r.URI == "/api/customers" {
			count++
			if !json.Valid(r.Body) {
				t.Fatal(r)
			}
		}
	}
	if count != 3 {
		t.Fatal(count)
	}
	// Cold-cache dry-run must fail without sending even a discovery request.
	t.Setenv("SLOWBOOKS_CACHE_DIR", t.TempDir())
	before := len(fake.Requests())
	code, out, _ = invoke(t, "", "api", "POST", "/api/customers", "--dry-run")
	if code != 2 || len(fake.Requests()) != before {
		t.Fatal(code, out)
	}
}
