package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestReviewWireRetryBudget(t *testing.T) {
	isolate(t)
	for _, method := range []string{"GET", "HEAD"} {
		var calls atomic.Int32
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := calls.Add(1)
			if n == 1 {
				_, _ = w.Write([]byte(`{}`))
				return
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
		}))
		c, err := New(context.Background(), Options{URL: s.URL})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = c.Do(context.Background(), "GET", "/api/prime", nil, ""); err != nil {
			t.Fatal(err)
		}
		_, err = c.Do(context.Background(), method, "/api/fail", nil, "")
		s.Close()
		if output.ExitCode(err) != 8 || calls.Load() != 3 {
			t.Errorf("%s: %d wire requests including prime, err %v", method, calls.Load(), err)
		}
	}
}
func TestReviewDataSizeAndPathErrors(t *testing.T) {
	isolate(t)
	// A sparse file keeps the regression cheap on disk while exceeding the cap.
	path := filepath.Join(t.TempDir(), "oversized.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate((64 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	_, err = ReadData(strings.NewReader(""), "@"+path)
	if output.ExitCode(err) != 2 || !strings.Contains(err.Error(), "64 MiB") {
		t.Errorf("oversize: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing.json")
	_, err = ReadData(strings.NewReader(""), "@"+missing)
	if output.ExitCode(err) != 2 || !strings.Contains(err.Error(), missing) {
		t.Errorf("missing file: %v", err)
	}
}

func TestReviewDataCapAllSources(t *testing.T) {
	isolate(t)
	// Valid JSON exactly at the cap succeeds; one extra byte must be rejected on
	// stdin and inline sources, not just @files.
	atCap := `"` + strings.Repeat("a", (64<<20)-2) + `"`
	for _, source := range []string{"-", atCap} {
		body, err := ReadData(strings.NewReader(atCap), source)
		if err != nil || len(body) != (64<<20) {
			t.Fatal("at cap", len(body), err)
		}
	}
	for _, source := range []string{"-", atCap + " "} {
		_, err := ReadData(strings.NewReader(atCap+" "), source)
		if output.ExitCode(err) != 2 || !strings.Contains(err.Error(), "64 MiB") {
			t.Fatal("over cap", err)
		}
	}
}
