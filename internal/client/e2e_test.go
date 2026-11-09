package client

import (
	"context"
	"encoding/json"
	"net"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

// TestE2E is opt-in; scripts/e2e.sh owns the disposable Docker stack and paths.
func TestE2E(t *testing.T) {
	server := os.Getenv("SBP_E2E_URL")
	if server == "" {
		t.Skip("run scripts/e2e.sh for the real-server gate")
	}
	requireLoopback(t, server)
	isolate(t)
	ctx := context.Background()
	c, err := New(ctx, Options{URL: server, Version: "e2e"})
	if err != nil {
		t.Fatal(err)
	}
	c.http.Jar, err = cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Generated for this disposable stack; never print the password or tokens.
	password := "sbp-e2e-disposable-password"
	payload, _ := json.Marshal(map[string]string{"password": password, "company_name": "CLI E2E"})
	if _, err := c.Do(ctx, "POST", "/api/auth/setup", payload, ""); err != nil {
		t.Fatal(err)
	}
	payload, _ = json.Marshal(map[string]string{"password": password})
	if _, err := c.Do(ctx, "POST", "/api/auth/login", payload, ""); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"bookkeeper", "readonly"} {
		payload, _ = json.Marshal(map[string]string{"label": "CLI E2E " + role, "role": role})
		data, err := c.Do(ctx, "POST", "/api/tokens", payload, "")
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(data, &result); err != nil || len(result.Token) < 5 || result.Token[:4] != "sbp_" {
			t.Fatal("invalid token response")
		}
		path := os.Getenv("SBP_E2E_TOKEN")
		if role == "readonly" {
			path = os.Getenv("SBP_E2E_READONLY_TOKEN")
		}
		if err := os.WriteFile(path, []byte(result.Token+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		tokenClient, err := New(ctx, Options{URL: server, Token: result.Token, Version: "e2e"})
		if err != nil {
			t.Fatal(err)
		}
		data, err = tokenClient.Do(ctx, "GET", "/api/system", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		var system struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &system); err != nil || system.Version != "2.31.10" {
			t.Fatalf("unexpected server version: %s", data)
		}
		if role == "bookkeeper" {
			w := output.New(os.Stdout, os.Stderr)
			if err := w.Text("server version " + system.Version + "\n"); err != nil {
				t.Fatal(err)
			}
		}
	}
	data, err := c.Do(ctx, "GET", "/openapi.json", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(raw, data, 0600); err != nil {
		t.Fatal(err)
	}
	// Match Python's canonical formatting exactly for shell diff and capture.
	formatted, err := exec.Command("python3", "-m", "json.tool", "--sort-keys", raw).Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("SBP_E2E_SNAPSHOT"), formatted, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptFake(t *testing.T) {
	server := os.Getenv("SBP_ACCEPT_URL")
	if server == "" {
		t.Skip("run scripts/accept.sh")
	}
	requireLoopback(t, server)
	isolate(t)
	c, err := New(context.Background(), Options{URL: server, Version: "accept"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := c.Do(context.Background(), "GET", "/api/system", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.Version != "2.31.10" {
		t.Fatalf("system: %s, %v", data, err)
	}
}

// Harness variables must never point an opt-in test at real books.
func requireLoopback(t *testing.T, server string) {
	t.Helper()
	parsed, err := url.Parse(server)
	if err != nil {
		t.Fatal(err)
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		t.Fatal("harness URL must use a literal loopback address")
	}
}
