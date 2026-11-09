package cli

import (
	"strings"
	"testing"
)

func TestNamedWritesRejectDuplicateAllocations(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	for _, kind := range []string{"payments", "bill-payments"} {
		party, id := "customer", "invoice_id"
		if kind == "bill-payments" {
			party, id = "vendor", "bill_id"
		}
		base := []string{kind, "record", "--key", "dup", "--" + party, "1", "--date", "2026-11-09", "--amount", "0.02"}
		for _, extra := range [][]string{
			{"--apply", "4=0.01", "--apply", "4=0.01"},
			{"--data", `{"allocations":[{"` + id + `":4,"amount":"0.01"},{"` + id + `":4,"amount":0.01}]}`},
		} {
			before := len(fake.Requests())
			code, out, _ := invoke(t, "", append(append([]string{}, base...), extra...)...)
			if code != 2 || !strings.Contains(out, "repeated allocation") || before != len(fake.Requests()) {
				t.Errorf("%v: %d %s", extra, code, out)
			}
		}
	}
}
