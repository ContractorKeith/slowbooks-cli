package cli

import (
	"strings"
	"testing"
)

func TestNamedWriteStringFlagsRefuseBlank(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	for _, flag := range []string{"due", "memo", "date"} {
		for _, blank := range []string{"", "  "} {
			before := len(fake.Requests())
			code, out, _ := invoke(t, "", "invoices", "create", "--customer", "1", "--date", "2026-11-09", "--"+flag, blank)
			if code != 2 || !strings.Contains(out, "must not be blank") || before != len(fake.Requests()) {
				t.Errorf("%s %q: %d %s", flag, blank, code, out)
			}
		}
	}
	code, out, _ := invoke(t, "", "invoices", "create", "--help")
	if code != 0 || !strings.Contains(out, "must not be blank") {
		t.Errorf("help: %d %s", code, out)
	}
}
