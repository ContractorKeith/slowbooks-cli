package client

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
		t.Fatal("closing_date setting absent")
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
	// Execute the CLI as a process to prove the same sentence and exit code survive.
	binary := filepath.Join(t.TempDir(), "sbp")
	build := exec.Command("go", "build", "-o", binary, "./cmd/sbp")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	cmd := exec.Command(binary, "invoices", "create", "--customer", customer.ID.String(), "--date", "2026-01-01", "--data", string(body), "--json")
	cmd.Env = append(os.Environ(), "SLOWBOOKS_URL="+os.Getenv("SBP_E2E_URL"), "SLOWBOOKS_TOKEN_FILE="+os.Getenv("SBP_E2E_TOKEN"))
	out, err := cmd.Output()
	var exit *exec.ExitError
	if err == nil {
		t.Fatal("closed write succeeded")
	}
	var ok bool
	exit, ok = err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 4 {
		t.Fatalf("exit: %v %s", err, out)
	}
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(out, &envelope) != nil || envelope.Error.Message != e.Message {
		t.Fatalf("message not preserved: %s", out)
	}
	t.Log("M5 closing-date write refused with exit 4 and the server sentence; setting restored at cleanup")
}
