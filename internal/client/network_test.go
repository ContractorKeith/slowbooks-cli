package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHTTPPinsCheckedAddresses(t *testing.T) {
	isolate(t)
	reached := false
	expectedHost := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		if r.URL.RequestURI() != "/api/system?limit=1" || r.Host != expectedHost {
			t.Errorf("unexpected target %s %s", r.Host, r.URL)
		}
		_, _ = w.Write([]byte(`{"version":"test"}`))
	}))
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	expectedHost = "checked.invalid:" + parsed.Port()
	calls := 0
	lookup := func(ctx context.Context, host string) ([]net.IPAddr, error) {
		calls++
		if host != "checked.invalid" {
			t.Errorf("host = %s", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	c, err := newClient(context.Background(), Options{URL: "http://checked.invalid:" + parsed.Port()}, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Do(context.Background(), "GET", "/api/system?limit=1", nil, ""); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !reached {
		t.Fatalf("lookups %d reached %v", calls, reached)
	}
	if c.http.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("HTTP transport must bypass proxies")
	}
}
