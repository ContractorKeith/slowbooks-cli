package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestUnsupportedPaymentKeysNeverSuggestReplay(t *testing.T) {
	isolate(t)
	for _, drop := range []bool{false, true} {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if drop {
				c, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				_ = c.Close()
				return
			}
			w.WriteHeader(504)
		}))
		cl, err := New(context.Background(), Options{URL: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/api/batch-payments", "/api/payments/5/replace?x=1", "/api/bill-payments/5/replace"} {
			_, err := cl.Do(context.Background(), "POST", path, []byte("{}"), "caller-key")
			e := output.AsError(err)
			if output.ExitCode(err) != 8 || e.Retryable || !strings.Contains(e.Hint, "check the books") || strings.Contains(e.Hint, "same --key") {
				t.Errorf("%s: %+v", path, e)
			}
		}
		srv.Close()
		if calls != 3 {
			t.Fatal(calls)
		}
	}
}
