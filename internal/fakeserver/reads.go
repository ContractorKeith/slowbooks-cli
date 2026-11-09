package fakeserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/testdata"
)

func customerFixtures() []json.RawMessage {
	raw, _ := testdata.Files.ReadFile("fake/customers.json")
	var rows []json.RawMessage
	_ = json.Unmarshal(raw, &rows)
	return rows
}

// readFixture serves the named read fixtures while retaining normal 404 behavior.
func (s *Server) readFixture(w http.ResponseWriter, r *http.Request) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	if path == "search" || path == "reports/profit-loss" {
		name := "search"
		if strings.HasPrefix(path, "reports/") {
			name = "profit-loss"
		}
		raw, _ := testdata.Files.ReadFile("fake/" + name + ".json")
		_, _ = w.Write(raw)
		return true
	}
	parts := strings.Split(path, "/")
	if len(parts) > 2 {
		return false
	}
	switch parts[0] {
	case "customers", "vendors", "items", "accounts", "invoices", "bills", "expenses", "payments", "bill-payments", "journal":
	default:
		return false
	}
	raw, _ := testdata.Files.ReadFile("fake/" + parts[0] + ".json")
	var rows []json.RawMessage
	_ = json.Unmarshal(raw, &rows)
	if len(parts) == 2 {
		for _, row := range rows {
			var obj map[string]any
			_ = json.Unmarshal(row, &obj)
			if id, ok := obj["id"].(float64); ok && strconv.Itoa(int(id)) == parts[1] {
				_, _ = w.Write(row)
				return true
			}
		}
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"detail":"Not found"}`))
		return true
	}
	if parts[0] == "invoices" || parts[0] == "bills" {
		filtered := make([]json.RawMessage, 0, len(rows))
		for _, row := range rows {
			var obj struct {
				Status string `json:"status"`
			}
			_ = json.Unmarshal(row, &obj)
			if status := r.URL.Query().Get("status"); status != "" && obj.Status != status {
				continue
			}
			if r.URL.Query().Get("open_only") == "true" && (obj.Status == "paid" || obj.Status == "void") {
				continue
			}
			filtered = append(filtered, row)
		}
		rows = filtered
	}
	if skip, err := strconv.Atoi(r.URL.Query().Get("skip")); err == nil && skip > 0 {
		rows = rows[min(skip, len(rows)):]
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n < len(rows) {
		rows = rows[:n]
	}
	_ = json.NewEncoder(w).Encode(rows)
	return true
}
