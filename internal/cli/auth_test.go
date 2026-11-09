package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/config"
	"github.com/ContractorKeith/slowbooks-cli/internal/fakeserver"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func setupAuth(t *testing.T) (*fakeserver.Server, string) {
	t.Helper()
	isolate(t)
	for _, k := range []string{"SLOWBOOKS_URL", "SLOWBOOKS_TOKEN_FILE", "SLOWBOOKS_COMPANY", "SLOWBOOKS_PROFILE", "SLOWBOOKS_TIMEOUT_SECONDS"} {
		t.Setenv(k, "")
	}
	fake := fakeserver.New()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return fake, srv.URL
}
func invoke(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	var out, err bytes.Buffer
	w := output.New(&out, &err)
	r := NewRoot(w, Build{"test", "test"})
	r.SetIn(strings.NewReader(input))
	code := Execute(r, w, args)
	if strings.Contains(out.String(), "sbp_") || strings.Contains(err.String(), "sbp_") {
		t.Fatal("token leaked")
	}
	return code, out.String(), err.String()
}
func TestAuthLifecycleAndRoleHeaders(t *testing.T) {
	fake, url := setupAuth(t)
	for _, r := range []string{"bookkeeper", "accountant", "readonly"} {
		code, out, _ := invoke(t, "sbp_"+r+"\n", "auth", "login", "--profile", r, "--url", url, "--company", "acme")
		if code != 0 || !strings.Contains(out, `"role":"`+r+`"`) {
			t.Fatal(code, out)
		}
	}
	code, out, _ := invoke(t, "", "auth", "status", "--profile", "readonly", "--company", "other")
	if code != 0 || !strings.Contains(out, `"company":"other"`) {
		t.Fatal(code, out)
	}
	reqs := fake.Requests()
	for _, r := range reqs[len(reqs)-2:] {
		if r.Header.Get("X-SlowBooks-Company") != "other" {
			t.Fatal(r)
		}
	}
	code, out, _ = invoke(t, "", "profile", "use", "readonly")
	if code != 0 {
		t.Fatal(out)
	}
	code, out, _ = invoke(t, "", "profile", "list")
	if code != 0 || !strings.Contains(out, `"default":"readonly"`) {
		t.Fatal(out)
	}
	code, out, _ = invoke(t, "", "auth", "logout")
	if code != 0 {
		t.Fatal(out)
	}
	code, out, _ = invoke(t, "", "auth", "status", "--profile", "readonly")
	if code != 3 {
		t.Fatal(code, out)
	}
}
func TestWrongTokenAndDoctor(t *testing.T) {
	_, url := setupAuth(t)
	code, out, _ := invoke(t, "sbp_wrong", "auth", "login", "--url", url)
	if code != 3 {
		t.Fatal(code, out)
	}
	st, _ := config.Load()
	if len(st.Profiles) != 0 {
		t.Fatal(st)
	}
	code, out, _ = invoke(t, "sbp_bookkeeper", "auth", "login", "--url", url)
	if code != 0 {
		t.Fatal(code, out)
	}
	code, out, _ = invoke(t, "", "doctor")
	if code != 0 {
		t.Fatal(code, out)
	}
	var result struct{ Data struct{ Checks []check } }
	if e := json.Unmarshal([]byte(out), &result); e != nil || len(result.Data.Checks) != 7 {
		t.Fatal(out, e)
	}
	st, _ = config.Load()
	path := st.Profiles[st.Default].TokenFile
	if e := os.WriteFile(path, []byte("sbp_wrong"), 0600); e != nil {
		t.Fatal(e)
	}
	code, out, _ = invoke(t, "", "doctor")
	if code != 3 || !strings.Contains(out, `"name":"reachability","ok":false`) {
		t.Fatal(code, out)
	}
}
func TestVersionWarnings(t *testing.T) {
	for _, version := range []string{"2.30.0", "2.99.0", "2.31.10"} {
		t.Run(version, func(t *testing.T) {
			_, _ = setupAuth(t)
			fake := fakeserver.New()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/system" {
					_, _ = w.Write([]byte(`{"version":"` + version + `"}`))
					return
				}
				fake.ServeHTTP(w, r)
			}))
			defer srv.Close()
			code, out, stderr := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", srv.URL)
			if code != 0 {
				t.Fatal(code, out)
			}
			if strings.Contains(stderr, "warning:") != (version != "2.31.10") {
				t.Fatal(stderr)
			}
		})
	}
}
func TestEnvOnly(t *testing.T) {
	_, url := setupAuth(t)
	path := t.TempDir() + "/token"
	if e := os.WriteFile(path, []byte("sbp_bookkeeper"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("SLOWBOOKS_URL", url)
	t.Setenv("SLOWBOOKS_TOKEN_FILE", path)
	for _, args := range [][]string{{"version"}, {"auth", "status"}, {"doctor"}} {
		code, out, _ := invoke(t, "", args...)
		if code != 0 {
			t.Fatal(code, out)
		}
	}
}

func TestProbeFailuresAreNotRoles(t *testing.T) {
	for _, path := range []string{"/api/audit", "/api/employees"} {
		t.Run(path, func(t *testing.T) {
			_, _ = setupAuth(t)
			fake := fakeserver.New()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == path {
					w.WriteHeader(503)
					_, _ = w.Write([]byte(`{"detail":"Unavailable"}`))
					return
				}
				fake.ServeHTTP(w, r)
			}))
			defer srv.Close()
			code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", srv.URL)
			if code != 8 {
				t.Fatal(code, out)
			}
			st, e := config.Load()
			if e != nil || len(st.Profiles) != 0 {
				t.Fatal(st, e)
			}
		})
	}
}
func TestDoctorMissingSetupAndCompanyFailure(t *testing.T) {
	_, _ = setupAuth(t)
	code, out, _ := invoke(t, "", "doctor")
	if code != 3 || !strings.Contains(out, `"name":"config file","ok":false`) {
		t.Fatal(code, out)
	}
	_, url := setupAuth(t)
	code, out, _ = invoke(t, "sbp_no_companies", "auth", "login", "--url", url)
	if code != 0 {
		t.Fatal(code, out)
	}
	code, out, _ = invoke(t, "", "doctor")
	if code != 4 || !strings.Contains(out, `"name":"companies","ok":false`) {
		t.Fatal(code, out)
	}
}
