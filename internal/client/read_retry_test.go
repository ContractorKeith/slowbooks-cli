package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestReadRetriesAndWriteAttempts(t *testing.T) {
	isolate(t)
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"} {
		for _, status := range []int{0, 500, 502, 503, 504} {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if calls.Load() == 1 {
					if status == 0 {
						conn, _, _ := w.(http.Hijacker).Hijack()
						_ = conn.Close()
						return
					}
					w.WriteHeader(status)
					return
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			c, _ := New(context.Background(), Options{URL: server.URL})
			_, err := c.Do(context.Background(), method, "/api/test", nil, "")
			read := method == "GET" || method == "HEAD"
			want := 1
			if read && status != 500 {
				want = 2
			}
			if int(calls.Load()) != want || (want == 2 && err != nil) {
				t.Fatalf("%s %d: calls %d err %v", method, status, calls.Load(), err)
			}
			server.Close()
		}
	}
}
func TestWriteTimeoutKeyAdvice(t *testing.T) {
	isolate(t)
	for _, key := range []string{"", "check-1"} {
		var calls atomic.Int32
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			<-r.Context().Done()
		}))
		c, _ := New(context.Background(), Options{URL: s.URL, Timeout: time.Second})
		_, err := c.ForOperation(true).Do(context.Background(), "POST", "/api/payments", nil, key)
		s.Close()
		e := output.AsError(err)
		if output.ExitCode(err) != 8 || e.Retryable != (key != "") || calls.Load() != 1 {
			t.Fatal(calls.Load(), e)
		}
		if key != "" && !strings.Contains(e.Hint, "same --key") || key == "" && !strings.Contains(e.Hint, "check the books") {
			t.Fatal(e)
		}
	}
}
