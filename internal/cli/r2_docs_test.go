package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestR2PowerShellInstructionsAvoidAliasAndSplatting(t *testing.T) {
	isolate(t)
	for _, path := range []string{"../../README.md", "../../skills/sbp/SKILL.md"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range []string{"Set-PSBreakpoint", `"@file"`, "sbp.exe invoices create"} {
			if !strings.Contains(string(raw), term) {
				t.Errorf("%s missing %s", path, term)
			}
		}
	}
}
func TestR2PlanReadFlagsMatchCLI(t *testing.T) {
	isolate(t)
	raw, err := os.ReadFile("../../docs/PLAN.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "sbp invoices|bills list") && strings.Contains(line, "--since") {
			t.Error(line)
		}
	}
	var buf bytes.Buffer
	root := NewRoot(output.New(&buf, &buf), Build{})
	for _, kind := range []string{"invoices", "bills"} {
		cmd, _, err := root.Find([]string{kind, "list"})
		if err != nil || cmd.Flags().Lookup("since") != nil {
			t.Fatal(kind, err)
		}
	}
}
