package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
)

func TestNamedWriteRequiredDate(t *testing.T) {
	fake, server := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", server); code != 0 {
		t.Fatal(out)
	}
	for _, row := range commands.Writes() {
		if len(row.Flags) == 0 {
			continue
		}
		if !slices.Contains(row.Required, "date") {
			t.Errorf("%s must declare date required", row.Name)
		}
		args := writeArgs(row)
		for i, arg := range args {
			if arg == "--date" {
				args = append(args[:i], args[i+2:]...)
				break
			}
		}
		before := len(fake.Requests())
		code, out, _ := invoke(t, "", args...)
		if code != 2 || !strings.Contains(out, "date") || len(fake.Requests()) != before {
			t.Errorf("%v: %d %s", args, code, out)
		}
	}
}
