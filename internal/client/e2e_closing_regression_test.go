package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestClosingDateAbsentSettingIsSetAndRestored(t *testing.T) {
	isolate(t)
	var mu sync.Mutex
	var updates []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/settings" && r.Method == "GET":
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/api/settings":
			raw, _ := readBody(r.Body)
			mu.Lock()
			updates = append(updates, string(raw))
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/api/customers":
			_, _ = w.Write([]byte(`{"id":1}`))
		case r.URL.Path == "/api/invoices":
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"detail":"Transaction date 2026-01-01 is on or before the closing date (2026-01-31). Modifications to closed periods are not allowed."}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("sbp_test"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SBP_E2E_URL", server.URL)
	t.Setenv("SBP_E2E_TOKEN", token)
	admin, err := New(context.Background(), Options{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("check", func(t *testing.T) { checkClosingDate(t, admin) })
	mu.Lock()
	defer mu.Unlock()
	if len(updates) != 2 || !strings.Contains(updates[0], "2026-01-31") || updates[1] != `{"closing_date":""}` {
		t.Fatal(updates)
	}
}
