package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestSkepticStatusAndOpen(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	// Warm catalog: rejected flags must send no operation request.
	invoke(t, "", "ops", "find", "invoice")
	for _, kind := range []string{"invoices", "bills"} {
		before := len(fake.Requests())
		code, out, _ := invoke(t, "", kind, "list", "--status", "open")
		if kind == "invoices" {
			if code != 2 || !strings.Contains(out, "draft") {
				t.Errorf("status: %d %s", code, out)
			}
			for _, r := range fake.Requests()[before:] {
				if strings.HasPrefix(r.URI, "/api/"+kind) {
					t.Error(r)
				}
			}
		} else if code != 0 {
			t.Errorf("undeclared bill status: %d %s", code, out)
		}
		code, out, _ = invoke(t, "", kind, "list", "--open")
		reqs := fake.Requests()
		if code != 0 || !strings.Contains(reqs[len(reqs)-1].URI, "open_only=true") {
			t.Errorf("open: %d %s", code, out)
		}
	}
}
func TestSkepticMissingCatalogKeyFallback(t *testing.T) {
	fake, server := setupAuth(t)
	invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	invoke(t, "", "ops", "find", "invoice")
	files, err := filepath.Glob(filepath.Join(os.Getenv("SLOWBOOKS_CACHE_DIR"), "default", "openapi-*.json"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	if err = os.WriteFile(files[0], []byte(`{"openapi":"3.1.0","paths":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	before := len(fake.Requests())
	for _, path := range []string{"/api/payments/5/discount", "/api/bill-payments/5/discount"} {
		code, out, _ := invoke(t, "", "api", "POST", path, "--data", "{}", "--dry-run")
		if code != 2 || !strings.Contains(out, "requires --key") {
			t.Errorf("%d %s", code, out)
		}
	}
	if len(fake.Requests()) != before {
		t.Fatal("offline HTTP")
	}
}
func TestSkepticUnknownCompany(t *testing.T) {
	fake, server := setupAuth(t)
	invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	for _, args := range [][]string{{"api", "POST", "/api/customers", "--data", "{}"}, {"invoices", "void", "1"}} {
		before := len(fake.Requests())
		code, out, _ := invoke(t, "", append(args, "--company", "bogus")...)
		if code != 2 || !strings.Contains(out, "acme") {
			t.Errorf("%d %s", code, out)
		}
		count := 0
		for _, r := range fake.Requests()[before:] {
			if r.Method != "GET" {
				t.Error("write sent", r)
			}
			if r.URI == "/api/companies" {
				count++
			}
		}
		if count != 1 {
			t.Error("company calls", count)
		}
	}
	code, out, _ := invoke(t, "", "doctor", "--company", "bogus")
	if code != 2 || !strings.Contains(out, `"name":"companies","ok":false`) {
		t.Errorf("%d %s", code, out)
	}
	code, out, _ = invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server, "--company", "bogus")
	if code != 2 {
		t.Errorf("login %d %s", code, out)
	}
}
func TestSkepticWarningsAndVersion(t *testing.T) {
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/api/system" {
			return false
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "9.9\x1b\a"})
		return true
	})
	code, out, stderr := invoke(t, "", "customers", "list")
	if code != 0 || strings.ContainsAny(stderr, "\x1b\a") || !strings.Contains(stderr, `\u001b`) {
		t.Fatal(code, out, stderr)
	}
	var buf bytes.Buffer
	if code := Run([]string{"version"}, &buf, &buf, Build{"v1.2.3", "abc"}); code != 0 || !strings.Contains(buf.String(), `"version":"1.2.3"`) {
		t.Fatal(buf.String())
	}
}
func TestSkepticNoPlaceholderHintsAndSingular(t *testing.T) {
	_, server := setupAuth(t)
	invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	code, out, _ := invoke(t, "", "invoices", "list")
	if code != 0 || strings.Contains(out, "<id>") || strings.Contains(out, `\u003cid`) || !strings.Contains(out, `"summary":"1 invoice"`) {
		t.Fatal(code, out)
	}
	for _, row := range commands.Reads() {
		if row.Name == "invoices pdf" && strings.Contains(strings.Join(row.Next, " "), "<id>") {
			t.Fatal(row.Next)
		}
	}
}
func TestSkepticEnvURLTokenGuard(t *testing.T) {
	fake, server := setupAuth(t)
	invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	t.Setenv("SLOWBOOKS_URL", "http://127.0.0.1:1")
	before := len(fake.Requests())
	code, out, _ := invoke(t, "", "customers", "list")
	if code != 3 || !strings.Contains(out, "SLOWBOOKS_TOKEN_FILE") || len(fake.Requests()) != before {
		t.Fatal(code, out)
	}
}
func TestSkepticDryRunHeadersAndKeys(t *testing.T) {
	fake, server := setupAuth(t)
	invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	invoke(t, "", "ops", "find", "customer")
	for _, flags := range [][]string{{"--company", "a\nb"}, {"--key", "a b"}, {"--key", strings.Repeat("a", 129)}} {
		before := len(fake.Requests())
		code, out, _ := invoke(t, "", append([]string{"api", "POST", "/api/customers", "--dry-run"}, flags...)...)
		if code != 2 || len(fake.Requests()) != before {
			t.Errorf("%d %s", code, out)
		}
	}
}
func TestSkepticTTYList(t *testing.T) {
	_, server := setupAuth(t)
	invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	var out, stderr bytes.Buffer
	w := output.New(&out, &stderr)
	w.TTY = true
	code := Execute(NewRoot(w, Build{}), w, []string{"expenses", "list", "--limit", "1"})
	if code != 0 || !strings.Contains(stderr.String(), "1 of 55 shown") {
		t.Fatal(code, out.String(), stderr.String())
	}
}

func TestSkepticMacInstallRecovery(t *testing.T) {
	isolate(t)
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"xattr -d com.apple.quarantine sbp", "gh release download", "curl"} {
		if !strings.Contains(string(raw), command) {
			t.Error("missing install instruction", command)
		}
	}
}
