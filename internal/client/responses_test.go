package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestRedirectMessageKeepsRefusalAndDetail(t *testing.T) {
	isolate(t)
	err := responseError(http.StatusFound, []byte(`{"detail":"go to another server"}`), true)
	if err.Message != "server redirect refused" || err.Code != "usage" {
		t.Fatalf("redirect: %+v", err)
	}
	detail, _ := json.Marshal(err.Detail)
	if string(detail) != `"go to another server"` {
		t.Fatalf("detail: %s", detail)
	}
}

func TestResponseSizeCap(t *testing.T) {
	isolate(t)
	body := "1234"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
	defer server.Close()
	c, err := New(context.Background(), Options{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	c.maxResponseBytes = 4
	data, err := c.Do(context.Background(), "GET", "/api/system", nil, "")
	if err != nil || string(data) != "1234" {
		t.Fatalf("at cap: %s, %v", data, err)
	}
	body = "12345"
	for _, method := range []string{"GET", "POST"} {
		data, err := c.Do(context.Background(), method, "/api/system", nil, "")
		if output.ExitCode(err) != 8 || err.Error() != "response too large" || data != nil {
			t.Fatalf("over cap: %s, %v", data, err)
		}
		if output.AsError(err).Retryable {
			t.Fatal("oversized responses cannot succeed by retrying")
		}
		if method == "POST" && !strings.Contains(output.AsError(err).Hint, "check the books") {
			t.Fatal("write lost safety hint")
		}
	}
}
