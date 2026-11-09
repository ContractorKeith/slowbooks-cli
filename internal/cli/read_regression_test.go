package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/fakeserver"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/ContractorKeith/slowbooks-cli/testdata"
)

func readServer(t *testing.T, handler func(http.ResponseWriter, *http.Request) bool) {
	t.Helper()
	setupAuth(t)
	fake := fakeserver.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !handler(w, r) {
			fake.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(server.Close)
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server.URL)
	if code != 0 {
		t.Fatal(out)
	}
}

func TestReportDeclaredParams(t *testing.T) {
	var query string
	readServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasPrefix(r.URL.Path, "/api/reports/") {
			query = r.URL.RawQuery
			fmt.Fprint(w, `{}`)
			return true
		}
		return false
	})
	for _, tc := range []struct{ report, flag, target string }{
		{"profit-loss", "from", "start_date"}, {"profit-loss", "to", "end_date"},
		{"ar-aging", "to", "as_of_date"}, {"ap-aging", "to", "as_of_date"},
		{"balance-sheet", "to", "as_of_date"}, {"statement-of-financial-position", "to", "as_of_date"},
		{"wip-schedule", "to", "as_of"},
	} {
		code, out, _ := invoke(t, "", "reports", "run", tc.report, "--"+tc.flag, "2026-01-31")
		if code != 0 || query != tc.target+"=2026-01-31" {
			t.Fatal(tc, code, out, query)
		}
	}
	for _, args := range [][]string{
		{"ar-aging", "--from", "2026-01-01"}, {"1099-summary", "--to", "2026-01-31"},
		{"profit-loss", "--param", "typo=1"},
	} {
		query = "unchanged"
		code, out, _ := invoke(t, "", append([]string{"reports", "run"}, args...)...)
		if code != 2 || query != "unchanged" || !strings.Contains(out, "declared query parameters") {
			t.Fatal(args, code, out, query)
		}
	}
	code, out, _ := invoke(t, "", "reports", "run", "1099-summary", "--param", "year=2026")
	if code != 0 || query != "year=2026" {
		t.Fatal(code, out, query)
	}
}

func TestServerLimitSignal(t *testing.T) {
	var limit int
	readServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/api/invoices" {
			return false
		}
		limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
		rows := make([]map[string]int, limit)
		for i := range rows {
			rows[i] = map[string]int{"id": i + 1}
		}
		json.NewEncoder(w).Encode(rows)
		return true
	})
	code, out, _ := invoke(t, "", "invoices", "list")
	var result struct {
		Data    []any
		Summary string
	}
	json.Unmarshal([]byte(out), &result)
	if code != 0 || limit != 51 || len(result.Data) != 50 || result.Summary != "50 of 50+ shown; --all for everything" {
		t.Fatal(code, limit, out)
	}
}

func TestAllPages(t *testing.T) {
	var skips []int
	readServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/api/invoices" {
			return false
		}
		limit := 500
		if value := r.URL.Query().Get("limit"); value != "" {
			limit, _ = strconv.Atoi(value)
		}
		skip, _ := strconv.Atoi(r.URL.Query().Get("skip"))
		skips = append(skips, skip)
		end := min(skip+limit, 1001)
		rows := []map[string]int{}
		for i := skip; i < end; i++ {
			rows = append(rows, map[string]int{"id": i + 1})
		}
		json.NewEncoder(w).Encode(rows)
		return true
	})
	code, out, _ := invoke(t, "", "invoices", "list", "--all")
	var result struct{ Data []any }
	json.Unmarshal([]byte(out), &result)
	if code != 0 || len(result.Data) != 1001 || fmt.Sprint(skips) != "[0 500 1000]" {
		t.Fatal(code, len(result.Data), skips)
	}
}

func TestLargeIDListShow(t *testing.T) {
	readServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/api/customers":
			fmt.Fprint(w, `[{"id":9007199254740993}]`)
			return true
		case "/api/customers/9007199254740993":
			fmt.Fprint(w, `{"id":9007199254740993}`)
			return true
		}
		return false
	})
	code, out, _ := invoke(t, "", "customers", "list", "--jq", ".data[0].id")
	if code != 0 || out != "9007199254740993\n" {
		t.Fatal(code, out)
	}
	code, out, _ = invoke(t, "", "customers", "show", strings.TrimSpace(out))
	if code != 0 || !strings.Contains(out, `"id":9007199254740993`) {
		t.Fatal(code, out)
	}
}

func TestSearchFlagsAndSummary(t *testing.T) {
	readServer(t, func(http.ResponseWriter, *http.Request) bool { return false })
	for _, flag := range []string{"--all", "--limit=1"} {
		code, out, _ := invoke(t, "", "search", "Acme", flag)
		if code != 2 {
			t.Fatal(flag, code, out)
		}
	}
	code, out, _ := invoke(t, "", "search", "Acme")
	if code != 0 || !strings.Contains(out, `"summary":"Search results for Acme"`) {
		t.Fatal(code, out)
	}
}

func TestColumnsClearedAfterNamedRead(t *testing.T) {
	readServer(t, func(http.ResponseWriter, *http.Request) bool { return false })
	var out, stderr bytes.Buffer
	w := output.New(&out, &stderr)
	w.TTY = true
	root := NewRoot(w, Build{})
	if code := Execute(root, w, []string{"customers", "list"}); code != 0 {
		t.Fatal(code, out.String())
	}
	if len(w.Columns) != 0 {
		t.Fatal("columns leaked", w.Columns)
	}
	out.Reset()
	if err := w.Success([]map[string]any{{"unrelated": "visible"}}, "", nil); err != nil || !strings.Contains(out.String(), "unrelated") {
		t.Fatal(out.String(), err)
	}
}

func TestNonPagingMaximumWarning(t *testing.T) {
	var sentLimit string
	readServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/openapi.json" {
			raw, _ := testdata.Files.ReadFile("openapi-2.31.10.json")
			var doc map[string]any
			json.Unmarshal(raw, &doc)
			op := doc["paths"].(map[string]any)["/api/invoices"].(map[string]any)["get"].(map[string]any)
			op["parameters"] = []any{map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"maximum": 2}}}
			json.NewEncoder(w).Encode(doc)
			return true
		}
		if r.URL.Path == "/api/invoices" {
			sentLimit = r.URL.Query().Get("limit")
			fmt.Fprint(w, `[{"id":1},{"id":2}]`)
			return true
		}
		return false
	})
	code, out, stderr := invoke(t, "", "invoices", "list", "--all")
	if code != 0 || sentLimit != "2" || !strings.Contains(stderr, "results may be incomplete") {
		t.Fatal(code, out, stderr, sentLimit)
	}
}

func TestAllOffsetPages(t *testing.T) {
	var offsets []int
	readServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/openapi.json" {
			raw, _ := testdata.Files.ReadFile("openapi-2.31.10.json")
			var doc map[string]any
			json.Unmarshal(raw, &doc)
			op := doc["paths"].(map[string]any)["/api/invoices"].(map[string]any)["get"].(map[string]any)
			op["parameters"] = []any{map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"maximum": 2}}, map[string]any{"name": "offset", "in": "query"}}
			json.NewEncoder(w).Encode(doc)
			return true
		}
		if r.URL.Path == "/api/invoices" {
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			offsets = append(offsets, offset)
			if r.URL.Query().Get("limit") != "2" {
				t.Error("incorrect maximum", r.URL)
			}
			if offset == 0 {
				fmt.Fprint(w, `[{"id":1},{"id":2}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
			return true
		}
		return false
	})
	code, out, _ := invoke(t, "", "invoices", "list", "--all")
	if code != 0 || fmt.Sprint(offsets) != "[0 2]" || !strings.Contains(out, `"id":2`) {
		t.Fatal(code, out, offsets)
	}
}
