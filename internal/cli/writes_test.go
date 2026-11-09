package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
)

func writeArgs(row commands.Row) []string {
	args := strings.Fields(row.Name)
	if len(row.Args) > 0 {
		return append(args, "1")
	}
	args = append(args, "--date", "2026-11-09")
	if row.Name == "bill-payments record" {
		args = append(args, "--vendor", "1")
	} else {
		args = append(args, "--customer", "1")
	}
	if strings.HasSuffix(row.Name, "record") {
		args = append(args, "--key", "check-1042", "--amount", "1234.50", "--apply", "1=0.30", "--apply", "2=1234.20")
	}
	return args
}

func TestNamedWrites(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server, "--company", "acme"); code != 0 {
		t.Fatal(out)
	}
	if code, out, _ := invoke(t, "", "ops", "find", "payment"); code != 0 {
		t.Fatal(out)
	}
	for _, row := range commands.Writes() {
		t.Run(row.Name, func(t *testing.T) {
			args := writeArgs(row)
			before := len(fake.Requests())
			code, out, _ := invoke(t, "", append(args, "--dry-run")...)
			if code != 0 || len(fake.Requests()) != before || strings.Contains(out, "sbp_bookkeeper") || !strings.Contains(out, "redacted") {
				t.Fatal(code, out)
			}
			code, out, _ = invoke(t, "", args...)
			if code != 0 || !strings.Contains(out, `"next":["slowbooks `+strings.Fields(row.Name)[0]+` show 1"]`) {
				t.Fatal(code, out)
			}
			reqs := fake.Requests()
			last := reqs[len(reqs)-1]
			expectedPath := row.Path
			for _, arg := range row.Args {
				expectedPath = strings.ReplaceAll(expectedPath, "{"+arg+"}", "1")
			}
			if last.URI != expectedPath {
				t.Fatal(last.URI, expectedPath)
			}
			if last.Method != "POST" || last.Header.Get("X-SlowBooks-Company") != "acme" {
				t.Fatal(last)
			}
			if strings.HasSuffix(row.Name, "record") {
				if last.Header.Get("Idempotency-Key") != "check-1042" || !strings.Contains(string(last.Body), `"amount":1234.50`) || !strings.Contains(string(last.Body), `"amount":0.30`) {
					t.Fatal(last)
				}
				code, out, _ = invoke(t, "", args...)
				if code != 0 {
					t.Fatal(code, out)
				}
				code, out, _ = invoke(t, "", append(args, "--amount", "1")...)
				if code != 6 {
					t.Fatal(code, out)
				}
			}
		})
	}
	code, out, _ := invoke(t, "", "payments", "void", "1", "--fee-refunded")
	reqs := fake.Requests()
	if code != 0 || reqs[len(reqs)-1].URI != "/api/payments/1/void?fee_refunded=true" {
		t.Fatal(code, out, reqs[len(reqs)-1])
	}
	for _, args := range [][]string{
		{"payments", "record"}, {"bill-payments", "record"},
		{"invoices", "create"}, {"invoices", "create", "--data", "[]"},
		{"payments", "record", "--key", "k", "--customer", "1", "--date", "2026-11-09", "--amount", "NaN"},
		{"payments", "record", "--key", "k", "--customer", "1", "--date", "2026-11-09", "--amount", "1", "--apply", "../1=2"},
		{"invoices", "void", "../1"}, {"bills", "void", "1", "--fee-refunded"},
	} {
		before := len(fake.Requests())
		code, out, _ = invoke(t, "", args...)
		if code != 2 || len(fake.Requests()) != before {
			t.Fatal(args, code, out)
		}
	}
	if code, out, _ = invoke(t, "sbp_readonly", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	code, out, _ = invoke(t, "", "invoices", "create", "--customer", "1", "--date", "2026-11-09")
	if code != 4 || !strings.Contains(out, `"message":"This token is read-only"`) {
		t.Fatal(code, out)
	}
}

func TestWriteDataOverridesAndSources(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	body := `{"customer_id":9,"date":"2020-01-01","amount":"1234.50","allocations":[{"invoice_id":9,"amount":0.30}]}`
	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	for i, source := range []string{body, "@" + file, "-"} {
		code, out, _ := invoke(t, body, "payments", "record", "--key", string(rune('a'+i)), "--data", source, "--customer", "9007199254740993", "--date", "2026-11-09")
		reqs := fake.Requests()
		raw := string(reqs[len(reqs)-1].Body)
		if code != 0 || !strings.Contains(raw, `"amount":"1234.50"`) || !strings.Contains(raw, `"amount":0.30`) || !strings.Contains(raw, `"customer_id":9007199254740993`) {
			t.Fatal(code, out, raw)
		}
	}
	code, out, _ := invoke(t, "", "invoices", "create", "--data", `{"customer_id":1,"date":"2026-01-01","notes":"old","lines":[]}`, "--memo", "new", "--due", "2026-12-01")
	reqs := fake.Requests()
	var fields map[string]any
	_ = json.Unmarshal(reqs[len(reqs)-1].Body, &fields)
	if code != 0 || fields["notes"] != "new" || fields["due_date"] != "2026-12-01" {
		t.Fatal(code, out, fields)
	}
}
