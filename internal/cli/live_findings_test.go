package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func invokeHuman(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	w := output.New(&out, &stderr)
	w.TTY = true
	code := Execute(NewRoot(w, Build{}), w, args)
	return code, out.String(), stderr.String()
}

func TestLiveBalanceColumns(t *testing.T) {
	readServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/invoices" || r.URL.Path == "/api/bills" {
			fmt.Fprint(w, `[{"id":1,"total":"0.00","balance_due":"125.50","extra":"preserved"}]`)
			return true
		}
		return false
	})
	for _, kind := range []string{"invoices", "bills"} {
		code, out, stderr := invokeHuman(t, kind, "list")
		if code != 0 || !strings.Contains(out, "balance_due") || !strings.Contains(out, "125.50") || strings.Index(out, "balance_due") < strings.Index(out, "total") {
			t.Fatal(code, out, stderr)
		}
		code, out, _ = invoke(t, "", kind, "list", "--json")
		if code != 0 || !strings.Contains(out, `"extra":"preserved"`) {
			t.Fatal(code, out)
		}
	}
}

func TestLivePaymentReference(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	if code, out, _ := invoke(t, "", "ops", "find", "payment"); code != 0 {
		t.Fatal(out)
	}
	for _, tc := range []struct{ kind, field string }{{"payments", "reference"}, {"bill-payments", "check_number"}} {
		for i, raw := range []string{`{}`, fmt.Sprintf(`{"%s":"custom"}`, tc.field), fmt.Sprintf(`{"%s":null}`, tc.field)} {
			party := "--customer"
			if tc.kind == "bill-payments" {
				party = "--vendor"
			}
			key := fmt.Sprintf("chk-%s-%d", tc.kind, i)
			args := []string{tc.kind, "record", "--key", key, party, "1", "--amount", "5", "--date", "2026-11-09", "--data", raw}
			want := any(key)
			if i == 1 {
				want = "custom"
			}
			if i == 2 {
				want = nil
			}
			before := len(fake.Requests())
			code, out, _ := invoke(t, "", append(args, "--dry-run")...)
			var preview struct{ Data struct{ Body map[string]any } }
			if json.Unmarshal([]byte(out), &preview) != nil || code != 0 || len(fake.Requests()) != before || preview.Data.Body[tc.field] != want {
				t.Fatal(code, out)
			}
			code, out, _ = invoke(t, "", args...)
			reqs := fake.Requests()
			last := reqs[len(reqs)-1]
			var body map[string]any
			if json.Unmarshal(last.Body, &body) != nil || code != 0 || body[tc.field] != want || last.Header.Get("Idempotency-Key") != key {
				t.Fatal(code, out, string(last.Body))
			}
		}
	}
}

func TestLiveVoidIdentity(t *testing.T) {
	var response string
	readServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/void") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, response)
			return true
		}
		return false
	})
	for _, row := range commands.Writes() {
		if !strings.HasSuffix(row.Name, " void") {
			continue
		}
		kind := strings.Fields(row.Name)[0]
		noun := singular(kind)
		if kind == "bill-payments" {
			noun = "bill payment"
		}
		for _, numbered := range []bool{false, true} {
			response = `{}`
			label := "30"
			if numbered {
				label = "DOC-1029"
				field := map[string]string{"invoices": "invoice_number", "bills": "bill_number", "expenses": "expense_number", "journal": "entry_number", "payments": "payment_number", "bill-payments": "payment_number"}[kind]
				response = fmt.Sprintf(`{"id":30,"%s":"%s"}`, field, label)
			}
			code, out, _ := invoke(t, "", kind, "void", "30", "--json")
			var result struct {
				Summary string
				Next    []string
			}
			want := "Voided " + noun + " " + label
			if json.Unmarshal([]byte(out), &result) != nil || code != 0 || result.Summary != want || len(result.Next) != 1 || result.Next[0] != "slowbooks "+kind+" show 30" {
				t.Fatal(code, out, want)
			}
			code, out, stderr := invokeHuman(t, kind, "void", "30")
			if code != 0 || !strings.Contains(out, want) {
				t.Fatal(code, out, stderr, want)
			}
		}
	}
}

func TestLiveOpsFindLimits(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	var total int
	for _, flags := range [][]string{{"--all"}, {}, {"--limit", "2"}, {"--limit", "99999"}} {
		args := append([]string{"ops", "find", "item", "--json"}, flags...)
		code, out, _ := invoke(t, "", args...)
		var result struct {
			OK      bool
			Data    []map[string]any
			Summary string
			Next    []string
		}
		if json.Unmarshal([]byte(out), &result) != nil || code != 0 || !result.OK || result.Next == nil {
			t.Fatal(code, out)
		}
		want := total
		if len(flags) > 0 && flags[0] == "--all" {
			total = len(result.Data)
			want = total
			if total <= 20 {
				t.Fatal("need >20 matches", total)
			}
		}
		if len(flags) == 0 {
			want = 20
		}
		if len(flags) > 1 && flags[1] == "2" {
			want = 2
		}
		if len(result.Data) != want {
			t.Fatal(flags, len(result.Data), want)
		}
		if want < total && result.Summary != fmt.Sprintf("%d of %d shown; --all for everything", want, total) {
			t.Fatal(out)
		}
		for _, op := range result.Data {
			if _, ok := op["params"]; ok {
				t.Fatal("find should leave schemas to describe", out)
			}
		}
	}
	for _, flags := range [][]string{{"--limit", "0"}, {"--limit", "-1"}, {"--all", "--limit", "2"}} {
		before := len(fake.Requests())
		code, out, _ := invoke(t, "", append([]string{"ops", "find", "item"}, flags...)...)
		if code != 2 || len(fake.Requests()) != before {
			t.Fatal(code, out)
		}
	}
	code, out, _ := invoke(t, "", "ops", "describe", "POST", "/api/payments", "--json")
	if code != 0 || !strings.Contains(out, `"body_schema":{`) || !strings.Contains(out, `"params":[]`) {
		t.Fatal(code, out)
	}
	code, out, _ = invoke(t, "", "ops", "find", "no-such-operation-xyz", "--json")
	if code != 0 || !strings.Contains(out, `"data":[]`) {
		t.Fatal(code, out)
	}
}
