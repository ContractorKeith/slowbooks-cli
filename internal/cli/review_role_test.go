package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/config"
	"github.com/ContractorKeith/slowbooks-cli/internal/fakeserver"
)

func TestAdvisoryRoleFailures(t *testing.T) {
	for _, path := range []string{"/api/audit", "/api/employees"} {
		for _, status := range []int{404, 500, 503} {
			t.Run(fmt.Sprintf("%s/%d", path, status), func(t *testing.T) {
				setupAuth(t)
				fake := fakeserver.New()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == path {
						w.WriteHeader(status)
						return
					}
					fake.ServeHTTP(w, r)
				}))
				defer server.Close()
				code, out, stderr := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server.URL)
				if code != 0 || !strings.Contains(out, `"role":"unknown"`) || !strings.Contains(stderr, "warning:") {
					t.Fatalf("login: %d %s %s", code, out, stderr)
				}
				saved, e := config.Load()
				if e != nil || saved.Profiles["default"].Role != "unknown" {
					t.Fatal(saved, e)
				}
				code, out, stderr = invoke(t, "", "auth", "status")
				if code != 0 || !strings.Contains(out, `"role":"unknown"`) || !strings.Contains(stderr, "warning:") {
					t.Fatalf("status: %d %s %s", code, out, stderr)
				}
			})
		}
	}
}

func TestTokenRoleIgnoresCompanyAndLoginForwardsCompany(t *testing.T) {
	setupAuth(t)
	fake := fakeserver.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path == "/api/audit" || r.URL.Path == "/api/employees") && r.Header.Get("X-SlowBooks-Company") == "wrong" {
			w.WriteHeader(403)
			return
		}
		fake.ServeHTTP(w, r)
	}))
	defer server.Close()
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server.URL, "--company", "wrong")
	reqs := fake.Requests()
	if len(reqs) == 0 || reqs[0].Header.Get("X-SlowBooks-Company") != "wrong" {
		t.Fatal("login did not forward company to system")
	}
	if code != 0 || !strings.Contains(out, `"role":"bookkeeper"`) {
		t.Fatal(code, out)
	}
	st, e := config.Load()
	if e != nil || st.Profiles["default"].Role != "bookkeeper" {
		t.Fatal(st, e)
	}
	code, out, _ = invoke(t, "", "auth", "status")
	if code != 0 || !strings.Contains(out, `"role":"bookkeeper"`) {
		t.Fatal(code, out)
	}
}

func TestLoginTokenLengthBoundary(t *testing.T) {
	for _, ending := range []string{"", "\n", "\r\n"} {
		t.Run(fmt.Sprintf("ending=%q", ending), func(t *testing.T) {
			setupAuth(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/system" {
					_, _ = w.Write([]byte(`{"version":"2.31.10"}`))
					return
				}
				_, _ = w.Write([]byte(`[]`))
			}))
			defer server.Close()
			token := "sbp_" + strings.Repeat("x", 4092)
			code, out, _ := invoke(t, token+ending, "auth", "login", "--url", server.URL)
			if code != 0 {
				t.Fatal(code, out)
			}
			code, out, _ = invoke(t, "", "auth", "status")
			if code != 0 {
				t.Fatal(code, out)
			}
			code, out, _ = invoke(t, token+"x", "auth", "login", "--url", server.URL)
			if code != 3 {
				t.Fatal(code, out)
			}
		})
	}
}
