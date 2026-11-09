package cli

import (
	"strings"
	"testing"
)

func TestNamedMoneyRejectsRoundingAndNonPlainDecimals(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	for _, kind := range []string{"payments", "bill-payments"} {
		party := "customer"
		id := "invoice_id"
		if kind == "bill-payments" {
			party = "vendor"
			id = "bill_id"
		}
		base := []string{kind, "record", "--key", "money", "--" + party, "1", "--date", "2026-11-09"}
		for _, amount := range []string{"1234.50499999999999999999", "0.004", "0.005", "1e2", "+1", "-1", "0", "0.00", "10000000000000", "01.00", ".50", "1.", " 1"} {
			cases := [][]string{
				append(append([]string{}, base...), "--amount", amount),
				append(append([]string{}, base...), "--amount", "1", "--apply", "4="+amount),
				append(append([]string{}, base...), "--data", `{"amount":"`+amount+`"}`),
				append(append([]string{}, base...), "--amount", "1", "--data", `{"allocations":[{"`+id+`":4,"amount":"`+amount+`"}]}`),
			}
			for _, args := range cases {
				before := len(fake.Requests())
				code, out, _ := invoke(t, "", args...)
				if code != 2 || len(fake.Requests()) != before || !strings.Contains(out, "positive plain decimal") {
					t.Errorf("%v: %d %s", args, code, out)
				}
			}
		}
		for _, amount := range []string{"0.004", "0.005", "1e2", "-1", "0", "10000000000000", "1234.50499999999999999999"} {
			for _, body := range []string{`{"amount":` + amount + `}`, `{"amount":1,"allocations":[{"` + id + `":4,"amount":` + amount + `}]}`} {
				before := len(fake.Requests())
				code, out, _ := invoke(t, "", append(append([]string{}, base...), "--data", body)...)
				if code != 2 || len(fake.Requests()) != before {
					t.Errorf("%s: %d %s", body, code, out)
				}
			}
		}
	}
}

func TestNamedMoneyAcceptsExactCentValues(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	if code, out, _ := invoke(t, "", "ops", "find", "payment"); code != 0 {
		t.Fatal(out)
	}
	for _, value := range []string{"0.01", "0.10", "1", "1.0", "1234.50", "9999999999999.99"} {
		before := len(fake.Requests())
		code, out, _ := invoke(t, "", "payments", "record", "--key", "cents", "--customer", "1", "--date", "2026-11-09", "--amount", value, "--apply", "4="+value, "--dry-run")
		if code != 0 || before != len(fake.Requests()) || !strings.Contains(out, `"amount":`+value) {
			t.Fatal(value, code, out)
		}
	}
}
