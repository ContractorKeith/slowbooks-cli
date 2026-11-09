package cli

import (
	"strings"
	"testing"
)

func TestR2BillsUsePinnedStatusFallback(t *testing.T) {
	fake := reviewServer(t, nil)
	before := len(fake.Requests())
	code, out, _ := invoke(t, "", "bills", "list", "--status", "open")
	if code != 2 || !strings.Contains(out, "draft, unpaid, partial, paid, void") {
		t.Fatal(code, out)
	}
	for _, r := range fake.Requests()[before:] {
		if strings.HasPrefix(r.URI, "/api/bills") {
			t.Error("invalid filter sent", r)
		}
	}
	code, out, stderr := invoke(t, "", "bills", "list", "--status", "unpaid")
	if code != 0 || strings.Contains(stderr, "does not declare status") {
		t.Fatal(code, out, stderr)
	}
}
