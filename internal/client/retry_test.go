package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

// A payment key also tells Go's transport a request is replayable. The client
// must override that behavior: a server that drops a write gets one attempt.
func TestKeyedWritesNeverReplayOnDroppedReusedConnection(t *testing.T) {
	isolate(t)
	for _, body := range [][]byte{nil, []byte(`{"amount":1}`)} {
		var writes atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{}`))
				return
			}
			writes.Add(1)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
		}))
		c, err := New(context.Background(), Options{URL: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Do(context.Background(), "GET", "/api/prime", nil, ""); err != nil {
			t.Fatal(err)
		}
		_, err = c.Do(context.Background(), "POST", "/api/payments", body, "check-1")
		server.Close()
		if output.ExitCode(err) != 8 || output.AsError(err).Retryable {
			t.Fatal(err)
		}
		if got := writes.Load(); got != 1 {
			t.Fatalf("write attempts = %d; want 1", got)
		}
	}
}
