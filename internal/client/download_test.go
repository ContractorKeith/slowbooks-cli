package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestDownloadHeadersRetryAndUncappedBody(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer sbp_test" || r.Header.Get("X-SlowBooks-Company") != "acme" || r.Header.Get("User-Agent") != "sbp/test" || r.URL.Query().Get("q") != "a&b" {
			t.Error(r)
		}
		if hits == 1 {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="old.pdf"; filename*=UTF-8''invoice%20one.pdf`)
		_, _ = io.CopyN(w, strings.NewReader(strings.Repeat("x", 1024)), 1024)
	}))
	defer server.Close()
	cl, err := New(context.Background(), Options{URL: server.URL, Token: "sbp_test", Company: "acme", Version: "test", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	cl.maxResponseBytes = 4
	var dst bytes.Buffer
	got, err := cl.Download(context.Background(), "/api/test?q=a%26b", &dst)
	if err != nil || hits != 2 || got.Bytes != 1024 || got.Status != 200 || got.Filename != "invoice one.pdf" || got.ContentType != "application/pdf" {
		t.Fatal(got, err, hits)
	}
}

func TestDownloadErrorsAndPartialBody(t *testing.T) {
	for _, status := range []int{302, 401, 403, 404, 409, 422, 500, 502} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			hits := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"detail":"fixture"}`)
			}))
			defer server.Close()
			cl, _ := New(context.Background(), Options{URL: server.URL})
			var dst bytes.Buffer
			_, err := cl.Download(context.Background(), "/api/test", &dst)
			want := responseError(status, []byte(`{"detail":"fixture"}`), true)
			if err == nil || output.AsError(err).Code != want.Code || dst.Len() != 0 {
				t.Fatal(err, dst.String())
			}
			expected := 1
			if status == 502 {
				expected = 2
			}
			if hits != expected {
				t.Fatal(hits)
			}
		})
	}
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "partial")
	}))
	defer server.Close()
	cl, _ := New(context.Background(), Options{URL: server.URL})
	var dst bytes.Buffer
	got, err := cl.Download(context.Background(), "/api/test", &dst)
	if err == nil || got.Bytes != 7 || hits != 1 || dst.String() != "partial" {
		t.Fatal(got, err, hits, dst.String())
	}
}

func TestDownloadRewindsStagedFile(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, "partial")
			return
		}
		_, _ = io.WriteString(w, "complete")
	}))
	defer server.Close()
	cl, _ := New(context.Background(), Options{URL: server.URL})
	dst, err := os.CreateTemp(t.TempDir(), "download-*")
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	got, err := cl.Download(context.Background(), "/api/test", dst)
	if err != nil || hits != 2 || got.Bytes != 8 {
		t.Fatal(got, err, hits)
	}
	_, _ = dst.Seek(0, io.SeekStart)
	data, _ := io.ReadAll(dst)
	if string(data) != "complete" {
		t.Fatal(string(data))
	}
}

type brokenDownloadWriter struct{}

func (brokenDownloadWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("disk full") }

func TestDownloadDoesNotRetryWriterFailures(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; _, _ = io.WriteString(w, "body") }))
	defer server.Close()
	cl, _ := New(context.Background(), Options{URL: server.URL})
	_, err := cl.Download(context.Background(), "/api/test", brokenDownloadWriter{})
	if err == nil || err.Error() != "disk full" || hits != 1 {
		t.Fatal(err, hits)
	}
}
