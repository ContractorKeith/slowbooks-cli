package fakeserver_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/fakeserver"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestFixturesAndRecording(t *testing.T) {
	for _, name := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(name, t.TempDir())
	}
	fake := fakeserver.New()
	server := httptest.NewServer(fake)
	defer server.Close()
	c, err := client.New(context.Background(), client.Options{URL: server.URL, Token: "sbp_test", Company: "acme", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/system", "/openapi.json"} {
		body, err := c.Do(context.Background(), "GET", path, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatal(err)
		}
		if path == "/api/system" && data["version"] != "2.31.10" {
			t.Fatal(data)
		}
		if path == "/openapi.json" && data["openapi"] == nil {
			t.Fatal("missing OpenAPI document")
		}
	}
	_, err = c.Do(context.Background(), "POST", "/api/system", []byte(`{"test":true}`), "")
	if output.ExitCode(err) != 1 {
		t.Fatal(err)
	}
	_, err = c.Do(context.Background(), "GET", "/api/missing", nil, "")
	if output.ExitCode(err) != 5 {
		t.Fatal(err)
	}
	requests := fake.Requests()
	if len(requests) != 4 || requests[0].Header.Get("Authorization") != "Bearer sbp_test" || requests[2].Method != "POST" || string(requests[2].Body) != `{"test":true}` {
		t.Fatal(requests)
	}
	requests[0].Header.Set("Authorization", "mutated")
	if fake.Requests()[0].Header.Get("Authorization") != "Bearer sbp_test" {
		t.Fatal("recorder snapshot aliases internal headers")
	}
}
