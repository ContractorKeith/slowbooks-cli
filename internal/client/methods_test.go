package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestMethodCaseAndAllowedMethods(t *testing.T) {
	isolate(t)
	var sent []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = append(sent, r.Method)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	c, err := New(context.Background(), Options{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"get", "head", "post", "put", "patch", "delete"} {
		_, err := c.Do(context.Background(), method, "/api/system", nil, "")
		if output.ExitCode(err) != 8 {
			t.Fatalf("%s: %v", method, err)
		}
		if output.AsError(err).Retryable != (method == "get" || method == "head") {
			t.Fatalf("%s: retryable %v", method, output.AsError(err).Retryable)
		}
		if got := sent[len(sent)-1]; got != strings.ToUpper(method) {
			t.Fatalf("sent %s; want uppercase %s", got, method)
		}
	}
	before := len(sent)
	for _, method := range []string{"", "OPTIONS", "TRACE", "CONNECT", "BREW", "GET /"} {
		_, err := c.Do(context.Background(), method, "/api/system", nil, "")
		if output.ExitCode(err) != 2 {
			t.Fatalf("method %q: %v", method, err)
		}
	}
	if len(sent) != before {
		t.Fatal("unsupported method reached the server")
	}
}
