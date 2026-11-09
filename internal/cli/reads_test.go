package cli

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
)

func TestNamedReads(t *testing.T) {
	fake, server := setupAuth(t)
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server, "--company", "acme")
	if code != 0 {
		t.Fatal(out)
	}
	for _, row := range commands.Reads() {
		args := strings.Fields(row.Name)
		for _, arg := range row.Args {
			value := "1"
			if arg == "name" {
				value = "profit-loss"
			}
			if arg == "q" {
				value = "Acme & Co"
			}
			args = append(args, value)
		}
		if row.Download {
			args = append(args, "-o", filepath.Join(t.TempDir(), "invoice.pdf"))
		}
		code, out, _ = invoke(t, "", args...)
		if code != 0 {
			t.Fatal(args, code, out)
		}
		var result struct {
			OK   bool
			Data any
			Next []string
		}
		if err := json.Unmarshal([]byte(out), &result); err != nil || !result.OK || result.Data == nil {
			t.Fatal(args, out, err)
		}
		if row.Catalog {
			if !strings.Contains(out, `"name":"profit-loss"`) {
				t.Fatal(out)
			}
			continue
		}
		reqs := fake.Requests()
		last := reqs[len(reqs)-1]
		if last.Method != "GET" || last.Header.Get("X-SlowBooks-Company") != "acme" || last.Header.Get("Authorization") != "Bearer sbp_bookkeeper" {
			t.Fatal(last)
		}
		parsed, err := url.Parse(last.URI)
		expectedPath := row.Path
		for _, arg := range row.Args {
			value := "1"
			if arg == "name" {
				value = "profit-loss"
			}
			expectedPath = strings.ReplaceAll(expectedPath, "{"+arg+"}", value)
		}
		if err != nil || parsed.Path != expectedPath {
			t.Fatal(last, expectedPath, err)
		}
		if row.Name == "search" && last.URI != "/api/search?q=Acme+%26+Co" {
			t.Fatal(last)
		}
	}
}
func TestReadLimitsAndQueryFlags(t *testing.T) {
	fake, server := setupAuth(t)
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	if code != 0 {
		t.Fatal(out)
	}
	for _, tc := range []struct {
		args          []string
		count         int
		limit, status string
	}{
		{[]string{"expenses", "list"}, 50, "", ""},
		{[]string{"expenses", "list", "--limit", "2"}, 2, "", ""},
		{[]string{"expenses", "list", "--all"}, 55, "", ""},
		{[]string{"invoices", "list", "--status", "sent"}, 1, "51", "sent"},
		{[]string{"bills", "list", "--limit", "2"}, 1, "3", ""},
		{[]string{"payments", "list", "--all"}, 1, "500", ""},
	} {
		code, out, _ = invoke(t, "", tc.args...)
		var result struct {
			Data    []any
			Summary string
		}
		if code != 0 || json.Unmarshal([]byte(out), &result) != nil || len(result.Data) != tc.count {
			t.Fatal(tc.args, code, out)
		}
		reqs := fake.Requests()
		u, _ := url.Parse(reqs[len(reqs)-1].URI)
		if u.Query().Get("limit") != tc.limit || u.Query().Get("status") != tc.status {
			t.Fatal(u)
		}
		if tc.args[0] == "expenses" && tc.count < 55 && !strings.Contains(result.Summary, "of 55 shown") {
			t.Fatal(out)
		}
	}
	code, out, _ = invoke(t, "", "reports", "run", "profit-loss", "--from", "2026-01-01", "--to", "2026-11-09", "--param", "basis=cash")
	reqs := fake.Requests()
	u, _ := url.Parse(reqs[len(reqs)-1].URI)
	if code != 0 || u.Query().Get("start_date") != "2026-01-01" || u.Query().Get("end_date") != "2026-11-09" || u.Query().Get("basis") != "cash" {
		t.Fatal(code, out, u)
	}
	// Search returns its server object without list flags.
	code, out, _ = invoke(t, "", "search", "Acme")
	if code != 0 || !strings.Contains(out, `"data":{"customers"`) {
		t.Fatal(code, out)
	}
	for _, args := range [][]string{{"expenses", "list", "--limit", "0"}, {"expenses", "list", "--all", "--limit", "2"}, {"invoices", "list", "--since", "2026-01-01"}, {"customers", "show", "../1"}, {"reports", "run", "../system"}} {
		before := len(fake.Requests())
		code, out, _ = invoke(t, "", args...)
		if code != 2 || len(fake.Requests()) != before {
			t.Fatal(args, code, out)
		}
	}
}

func TestJQAtRoot(t *testing.T) {
	fake, server := setupAuth(t)
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	if code != 0 {
		t.Fatal(out)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"customers", "list", "--jq", ".data[].id"}, "1\n"},
		{[]string{"customers", "list", "--quiet", "--jq", ".[].id"}, "1\n"},
		{[]string{"version", "--jq", ".data.version, .ok"}, "\"test\"\ntrue\n"},
		{[]string{"version", "--jq", "empty"}, ""},
	} {
		code, out, _ = invoke(t, "", tc.args...)
		if code != 0 || out != tc.want {
			t.Fatal(tc.args, code, out)
		}
	}
	for _, expr := range []string{"[", "missing_function", ""} {
		before := len(fake.Requests())
		code, out, _ = invoke(t, "", "customers", "list", "--jq", expr)
		if code != 2 || len(fake.Requests()) != before || !strings.Contains(out, `"code":"usage"`) {
			t.Fatal(code, out)
		}
	}
	code, out, _ = invoke(t, "", "customers", "show", "9999", "--quiet", "--jq", ".")
	if code != 5 || !strings.Contains(out, `"error"`) {
		t.Fatal(code, out)
	}
}

func TestNamedIDCannotInjectQuery(t *testing.T) {
	fake, server := setupAuth(t)
	code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server)
	if code != 0 {
		t.Fatal(out)
	}
	for _, id := range []string{"1?limit=9", "1#fragment"} {
		before := len(fake.Requests())
		code, out, _ = invoke(t, "", "customers", "show", id)
		if code != 2 || len(fake.Requests()) != before {
			t.Fatal(code, out, fake.Requests()[before:])
		}
	}
}

func TestJQEvaluationError(t *testing.T) {
	isolate(t)
	code, out, _ := invoke(t, "", "version", "--jq", `error("bad filter")`)
	if code != 2 || !strings.Contains(out, `"code":"usage"`) || !strings.Contains(out, "bad filter") {
		t.Fatal(code, out)
	}
}
