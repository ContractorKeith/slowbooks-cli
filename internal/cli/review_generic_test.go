package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/fakeserver"
)

// reviewServer logs in to an isolated fake, allowing focused response overrides.
func reviewServer(t *testing.T, override func(http.ResponseWriter, *http.Request) bool) *fakeserver.Server {
	t.Helper()
	fake, _ := setupAuth(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if override != nil && override(w, r) {
			return
		}
		fake.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server.URL)
	if code != 0 {
		t.Fatal(code, out)
	}
	code, out, _ = invoke(t, "", "ops", "find", "customer")
	if code != 0 {
		t.Fatal(code, out)
	}
	return fake
}
func TestReviewTrailingSlashKeyGuard(t *testing.T) {
	fake := reviewServer(t, nil)
	for _, path := range []string{"/api/payments/", "/api/bill-payments/", "/api/payments/1/discount/"} {
		for _, dry := range []bool{false, true} {
			args := []string{"api", "POST", path, "--data", "{}"}
			if dry {
				args = append(args, "--dry-run")
			}
			code, out, _ := invoke(t, "", args...)
			if code != 2 || !strings.Contains(out, "--key") {
				t.Errorf("%v: %d %s", args, code, out)
			}
		}
	}
	for _, r := range fake.Requests() {
		if r.Method == "POST" {
			t.Fatal("unguarded write sent", r)
		}
	}
}
func TestReviewInvalidKeys(t *testing.T) {
	fake := reviewServer(t, nil)
	for _, key := range []string{"", "a\nb", "a\rb", "a\tb", "a\x00b", "a\x7fb", "  check  ", strings.Repeat("k", 201)} {
		for _, dry := range []bool{false, true} {
			args := []string{"api", "POST", "/api/customers", "--key", key, "--data", "{}"}
			if dry {
				args = append(args, "--dry-run")
			}
			code, out, _ := invoke(t, "", args...)
			if code != 2 {
				t.Errorf("key %q dry %v: %d %s", key, dry, code, out)
			}
		}
	}
	for _, r := range fake.Requests() {
		if r.Method == "POST" {
			t.Fatal("invalid key sent", r)
		}
	}
}
func TestReviewMalformedQueries(t *testing.T) {
	fake := reviewServer(t, nil)
	for _, query := range []string{"?status=%ZZ&limit=1", "?a=1;b=2"} {
		for _, dry := range []bool{false, true} {
			args := []string{"api", "GET", "/api/customers" + query}
			if dry {
				args = append(args, "--dry-run")
			}
			code, out, _ := invoke(t, "", args...)
			if code != 2 {
				t.Errorf("%v: %d %s", args, code, out)
			}
		}
	}
	for _, r := range fake.Requests() {
		if strings.HasPrefix(r.URI, "/api/customers") {
			t.Fatal("bad filter sent", r)
		}
	}
}
func TestReviewPathParamRetargeting(t *testing.T) {
	fake := reviewServer(t, nil)
	for _, value := range []string{"a/b", "", ".", "..", "a\\b", "%2f", "a\x00b"} {
		for _, dry := range []bool{false, true} {
			args := []string{"api", "GET", "/api/customers/{customer_id}", "--param", "customer_id=" + value}
			if dry {
				args = append(args, "--dry-run")
			}
			code, out, _ := invoke(t, "", args...)
			if code != 2 {
				t.Errorf("value %q dry %v: %d %s", value, dry, code, out)
			}
		}
	}
	for _, r := range fake.Requests() {
		if strings.HasPrefix(r.URI, "/api/customers") {
			t.Fatal("retargeted request", r)
		}
	}
}
func TestReviewFileWritesAndPlainText(t *testing.T) {
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "POST" && (r.URL.Path == "/api/payroll/1/nacha" || r.URL.Path == "/api/csv/import/test") {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("committed\n"))
			return true
		}
		return false
	})
	for _, path := range []string{"/api/payroll/1/nacha", "/api/csv/import/test"} {
		code, out, _ := invoke(t, "", "api", "POST", path, "--data", "{}")
		if code != 0 || !strings.Contains(out, "committed") {
			t.Errorf("%s: %d %s", path, code, out)
		}
	}
	code, out, _ := invoke(t, "", "api", "GET", "/api/payroll/{run_id}/nacha", "--param", "run_id=1")
	if code != 2 || !strings.Contains(out, "sbp download") {
		t.Fatal(code, out)
	}
}
func TestReviewOddServerVersions(t *testing.T) {
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/system" {
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "2.32.0+build7"})
			return true
		}
		return false
	})
	code, out, stderr := invoke(t, "", "ops", "find", "customer")
	if code != 0 || !strings.Contains(out, "/api/customers") || !strings.Contains(stderr, "warning") {
		t.Fatal(code, out, stderr)
	}
}
func TestReviewMalformedJSONExit(t *testing.T) {
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/customers" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("broken"))
			return true
		}
		return false
	})
	for _, method := range []string{"GET", "POST"} {
		code, out, _ := invoke(t, "", "api", method, "/api/customers")
		if code != 1 || !strings.Contains(out, `"code":"error"`) {
			t.Errorf("%s: %d %s", method, code, out)
		}
	}
}
func TestReviewDisposableCacheFailure(t *testing.T) {
	reviewServer(t, nil)
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLOWBOOKS_CACHE_DIR", path)
	code, out, stderr := invoke(t, "", "ops", "find", "customer")
	if code != 0 || !strings.Contains(out, "/api/customers") || !strings.Contains(stderr, "warning:") || !strings.Contains(stderr, "cache") {
		t.Fatal(code, out, stderr)
	}
}

func TestReviewMalformedCatalogAndCacheExit(t *testing.T) {
	fake, _ := setupAuth(t)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			_, _ = w.Write([]byte(`{"broken":true}`))
			return
		}
		fake.ServeHTTP(w, r)
	}))
	defer s.Close()
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", s.URL)
	if code != 0 {
		t.Fatal(out)
	}
	code, out, _ = invoke(t, "", "ops", "find", "customer")
	if code != 1 || !strings.Contains(out, `"code":"error"`) {
		t.Fatal(code, out)
	}
	// A corrupt offline cache is a local failure, not a server/network class.
	cache := os.Getenv("SLOWBOOKS_CACHE_DIR")
	dir := filepath.Join(cache, "default")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"current": "2.31.10", "openapi-2.31.10.json": "broken"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	code, out, _ = invoke(t, "", "api", "GET", "/api/customers", "--dry-run")
	if code != 1 || !strings.Contains(out, `"code":"error"`) {
		t.Fatal(code, out)
	}
}
