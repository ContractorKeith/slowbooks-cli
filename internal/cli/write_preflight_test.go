package cli

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestNamedWriteLookupFailureDoesNotSuggestCheckingBooks(t *testing.T) {
	_, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	t.Setenv("SLOWBOOKS_URL", "http://lookup.invalid")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errout bytes.Buffer
	w := output.New(&out, &errout)
	root := NewRoot(w, Build{"test", "test"})
	root.SetContext(ctx)
	code := Execute(root, w, []string{"invoices", "create", "--customer", "1", "--date", "2026-11-09"})
	if code != 8 || strings.Contains(out.String(), "before running") || !strings.Contains(out.String(), "check the server URL and connection") {
		t.Fatal(code, out.String())
	}
}

func TestNamedWriteDiscoveryFailureDoesNotSuggestCheckingBooks(t *testing.T) {
	var fail atomic.Bool
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if !fail.Load() || r.URL.Path != "/api/system" {
			return false
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
		} else {
			_ = conn.Close()
		}
		return true
	})
	fail.Store(true)
	code, out, _ := invoke(t, "", "invoices", "create", "--customer", "1", "--date", "2026-11-09")
	if code != 8 || strings.Contains(out, "before running") || !strings.Contains(out, "check the server URL and connection") {
		t.Fatal(code, out)
	}
}
