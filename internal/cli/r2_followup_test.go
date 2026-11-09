package cli

import (
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/config"
)

func TestReviewCompanyLockoutHasPersistentRecovery(t *testing.T) {
	var served atomic.Bool
	served.Store(true)
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/companies" && !served.Load() {
			_, _ = w.Write([]byte(`[]`))
			return true
		}
		return false
	})
	initial, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	server := initial.Profiles[initial.Default].URL
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server, "--profile", "work", "--company", "acme")
	if code != 0 {
		t.Fatal(code, out)
	}
	if code, out, _ := invoke(t, "", "profile", "use", "work"); code != 0 {
		t.Fatal(code, out)
	}
	t.Setenv("SLOWBOOKS_COMPANY", "bogus")
	code, out, _ = invoke(t, "", "customers", "list")
	if code != 2 || !strings.Contains(out, "SLOWBOOKS_COMPANY") {
		t.Error(code, out)
	}
	t.Setenv("SLOWBOOKS_COMPANY", "")
	served.Store(false)
	code, out, _ = invoke(t, "", "customers", "list")
	if code != 2 || !strings.Contains(out, "slowbooks auth login --profile work --company=") {
		t.Error(code, out)
	}
	code, out, _ = invoke(t, "sbp_bookkeeper", "auth", "login", "--profile", "work", "--company=")
	if code != 0 {
		t.Fatal(code, out)
	}
	store, err := config.Load()
	if err != nil || store.Profiles["work"].Company != "" {
		t.Fatal(store, err)
	}
	code, out, _ = invoke(t, "", "customers", "list")
	if code != 0 {
		t.Fatal(code, out)
	}
}
func TestReviewKeyWarningOnlyForWrites(t *testing.T) {
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "HEAD" {
			w.WriteHeader(204)
			return true
		}
		return false
	})
	for _, method := range []string{"GET", "HEAD"} {
		code, out, stderr := invoke(t, "", "api", method, "/api/customers", "--key", "ignored")
		if code != 0 || strings.Contains(stderr, "server ignores --key") {
			t.Fatal(code, out, stderr)
		}
	}
}
func TestReviewSkillCompanyPolicyIsConsistent(t *testing.T) {
	isolate(t)
	raw, err := os.ReadFile("../../skills/slowbooks-cli/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "Login and writes refuse unknown companies") || strings.Contains(text, "or writes; a successful lookup still refuses unknown companies") {
		t.Fatal("stale company policy remains")
	}
	if !strings.Contains(text, "Online reads and") || !strings.Contains(text, "exact served-company `file`") {
		t.Fatal("missing authoritative company policy")
	}
}

func TestReviewRawJSONModesDocumented(t *testing.T) {
	isolate(t)
	raw, err := os.ReadFile("../../docs/json.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "`--json` and `--jq` emit raw JSON") {
		t.Fatal("raw JSON mode contract missing")
	}
}
