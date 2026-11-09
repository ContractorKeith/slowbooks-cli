package client

import (
	"context"
	"encoding/json"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

// checkClosingDate uses the bootstrap's administrator session, never a token, for settings.
func checkClosingDate(t *testing.T, admin *Client) {
	t.Helper()
	ctx := context.Background()
	raw, err := admin.Do(ctx, "GET", "/api/settings", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]json.RawMessage
	if err = json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	original := settings["closing_date"]
	if original == nil {
		original = json.RawMessage(`""`)
	}
	restore, _ := json.Marshal(map[string]json.RawMessage{"closing_date": original})
	t.Cleanup(func() {
		if _, err := admin.Do(ctx, "PUT", "/api/settings", restore, ""); err != nil {
			t.Error(err)
		}
	})
	if _, err = admin.Do(ctx, "PUT", "/api/settings", []byte(`{"closing_date":"2026-01-31"}`), ""); err != nil {
		t.Fatal(err)
	}
	raw, err = admin.Do(ctx, "POST", "/api/customers", []byte(`{"name":"M5 Closing Customer"}`), "")
	if err != nil {
		t.Fatal(err)
	}
	var customer struct {
		ID json.Number `json:"id"`
	}
	if err = json.Unmarshal(raw, &customer); err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(os.Getenv("SBP_E2E_TOKEN"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(ctx, Options{URL: os.Getenv("SBP_E2E_URL"), Token: strings.TrimSpace(string(token)), Version: "e2e"})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"customer_id":` + customer.ID.String() + `,"date":"2026-01-01","lines":[{"description":"Closed","quantity":"1","rate":"10","amount":"10"}]}`)
	_, err = c.Do(ctx, "POST", "/api/invoices", body, "")
	e := output.AsError(err)
	if err == nil || e.Code != "refused" || !strings.Contains(e.Message, "2026-01-31") {
		t.Fatalf("closing date refusal: %v", err)
	}
	t.Log("M5 closing-date HTTP refusal preserves the server sentence; setting restored at cleanup")
}

// TestE2EClosingDate gives the shell harness HTTP-only admin setup and teardown.
func TestE2EClosingDate(t *testing.T) {
	action := os.Getenv("SBP_E2E_CLOSING_ACTION")
	if action == "" {
		t.Skip("run scripts/e2e.sh")
	}
	server := os.Getenv("SBP_E2E_URL")
	requireLoopback(t, server)
	isolate(t)
	ctx := context.Background()
	admin, err := New(ctx, Options{URL: server, Version: "e2e"})
	if err != nil {
		t.Fatal(err)
	}
	admin.http.Jar, err = cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Do(ctx, "POST", "/api/auth/login", []byte(`{"password":"slowbooks-e2e-disposable-password"}`), ""); err != nil {
		t.Fatal(err)
	}
	state := os.Getenv("SBP_E2E_CLOSING_STATE")
	switch action {
	case "set":
		raw, err := admin.Do(ctx, "GET", "/api/settings", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		var settings map[string]json.RawMessage
		if err = json.Unmarshal(raw, &settings); err != nil {
			t.Fatal(err)
		}
		original := settings["closing_date"]
		if original == nil {
			original = json.RawMessage(`""`)
		}
		restore, _ := json.Marshal(map[string]json.RawMessage{"closing_date": original})
		if err = os.WriteFile(state, restore, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = admin.Do(ctx, "PUT", "/api/settings", []byte(`{"closing_date":"2026-01-31"}`), ""); err != nil {
			t.Fatal(err)
		}
	case "restore":
		restore, err := os.ReadFile(state)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = admin.Do(ctx, "PUT", "/api/settings", restore, ""); err != nil {
			t.Fatal(err)
		}
		if err = os.Remove(state); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("invalid closing-date action")
	}
}
