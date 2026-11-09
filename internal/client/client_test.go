package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func isolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, path)
	}
}

func TestStatusAndHeaders(t *testing.T) {
	isolate(t)
	cases := []struct {
		status, exit int
		code         string
		retryable    bool
	}{
		{200, 0, "", false}, {204, 0, "", false}, {301, 2, "usage", false}, {400, 7, "invalid", false}, {401, 3, "auth", false}, {403, 4, "refused", false},
		{404, 5, "not_found", false}, {409, 6, "conflict", false}, {422, 7, "invalid", false}, {429, 1, "error", false}, {500, 8, "server", false}, {501, 8, "server", false}, {502, 8, "server", true}, {503, 8, "server", true}, {504, 8, "server", true},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			for _, method := range []string{"GET", "POST"} {
				calls := 0
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					for key, want := range map[string]string{"Authorization": "Bearer sbp_test", "X-SlowBooks-Company": "acme", "User-Agent": "sbp/test", "Idempotency-Key": "check-1"} {
						if r.Header.Get(key) != want {
							t.Errorf("%s = %q", key, r.Header.Get(key))
						}
					}
					w.WriteHeader(tc.status)
					if tc.status != 204 {
						_, _ = w.Write([]byte(`{"detail":[{"loc":["amount"],"msg":"invalid"}]}`))
					}
				}))
				c, err := New(context.Background(), Options{URL: s.URL, Token: "sbp_test", Company: "acme", Version: "test"})
				if err != nil {
					t.Fatal(err)
				}
				_, err = c.Do(context.Background(), method, "/api/system", nil, "check-1")
				s.Close()
				if output.ExitCode(err) != tc.exit {
					t.Fatalf("exit = %d, err = %v", output.ExitCode(err), err)
				}
				wantCalls := 1
				if method == "GET" && tc.retryable {
					wantCalls = 2
				}
				if calls != wantCalls {
					t.Fatalf("attempts = %d", calls)
				}
				if err != nil {
					e := output.AsError(err)
					if e.Code != tc.code || e.Retryable != (tc.retryable && method == "GET") {
						t.Fatalf("error = %+v", e)
					}
					detail, _ := json.Marshal(e.Detail)
					if string(detail) != `[{"loc":["amount"],"msg":"invalid"}]` {
						t.Fatalf("detail = %s", detail)
					}
				}
			}
		})
	}
}

func TestStringDetail(t *testing.T) {
	isolate(t)
	e := responseError(403, []byte(`{"detail":"Books closed through October"}`), false)
	if e.Message != "Books closed through October" {
		t.Fatal(e)
	}
	detail, _ := json.Marshal(e.Detail)
	if string(detail) != `"Books closed through October"` {
		t.Fatal(string(detail))
	}
}

func TestNetworkRules(t *testing.T) {
	isolate(t)
	for _, address := range []string{"127.0.0.1", "10.1.2.3", "172.16.1.1", "192.168.1.1", "169.254.1.1", "::1", "fe80::1", "fd00::1"} {
		lookup := func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP(address)}}, nil
		}
		if _, err := newClient(context.Background(), Options{URL: "http://test.invalid"}, lookup); err != nil {
			t.Fatal(address, err)
		}
	}
	for _, addresses := range [][]net.IPAddr{nil, {{IP: net.ParseIP("8.8.8.8")}}, {{IP: net.ParseIP("127.0.0.1")}, {IP: net.ParseIP("8.8.8.8")}}} {
		lookup := func(context.Context, string) ([]net.IPAddr, error) { return addresses, nil }
		if _, err := newClient(context.Background(), Options{URL: "http://public.invalid"}, lookup); output.ExitCode(err) != 2 {
			t.Fatal(err)
		}
	}
	badLookup := func(context.Context, string) ([]net.IPAddr, error) { return nil, errors.New("DNS unavailable") }
	if _, err := newClient(context.Background(), Options{URL: "http://unknown.invalid"}, badLookup); output.ExitCode(err) != 2 {
		t.Fatal(err)
	}
	for _, raw := range []string{"ftp://localhost", "http://user:secret@localhost", "http://localhost/base", "http://localhost?x=1", "http://localhost#x", "://bad"} {
		if _, err := New(context.Background(), Options{URL: raw}); output.ExitCode(err) != 2 {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	c, err := New(context.Background(), Options{URL: "https://public.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/api", "/openapi.json/x", "//other/api/a", "https://other/api/a", "/api/../auth", "/api/%2e%2e/auth", "/api/%252e%252e/auth", "/api/a\\b", "/api//a", "/api/a#fragment"} {
		if _, err := c.Do(context.Background(), "GET", path, nil, ""); output.ExitCode(err) != 2 {
			t.Fatalf("%s: %v", path, err)
		}
	}
	for _, path := range []string{"/api/system", "/openapi.json", "/api/customers?limit=1"} {
		if _, err := requestURL(c.base, path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRedirectNeverReachesOtherHost(t *testing.T) {
	isolate(t)
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/api/system", http.StatusFound)
	}))
	defer redirect.Close()
	c, _ := New(context.Background(), Options{URL: redirect.URL, Token: "sbp_secret"})
	_, err := c.Do(context.Background(), "GET", "/api/system", nil, "")
	if output.ExitCode(err) != 2 || reached {
		t.Fatalf("redirect: %v, target reached: %v", err, reached)
	}
}

func TestTimeoutConnectionAndTLS(t *testing.T) {
	isolate(t)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	defer s.Close()
	for _, method := range []string{"GET", "POST"} {
		c, _ := New(context.Background(), Options{URL: s.URL, Timeout: time.Second})
		_, err := c.Do(context.Background(), method, "/api/system", nil, "")
		if output.ExitCode(err) != 8 || output.AsError(err).Retryable != (method == "GET") {
			t.Fatal(err)
		}
	}
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead.Close()
	c, _ := New(context.Background(), Options{URL: dead.URL})
	if _, err := c.Do(context.Background(), "GET", "/api/system", nil, ""); output.ExitCode(err) != 8 {
		t.Fatal(err)
	}
	tls := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer tls.Close()
	c, _ = New(context.Background(), Options{URL: tls.URL})
	if _, err := c.Do(context.Background(), "GET", "/api/system", nil, ""); output.ExitCode(err) != 8 {
		t.Fatal(err)
	}
	for _, timeout := range []time.Duration{-1, time.Millisecond, 3601 * time.Second} {
		if _, err := New(context.Background(), Options{URL: strings.Replace(s.URL, "http:", "https:", 1), Timeout: timeout}); output.ExitCode(err) != 2 {
			t.Fatal(err)
		}
	}
}
