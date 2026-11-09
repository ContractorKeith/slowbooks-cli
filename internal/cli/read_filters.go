package cli

import (
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func validateStatus(cat *catalog.Catalog, op catalog.Operation, value string) error {
	values := cat.QueryEnum(op, "status")
	// The pinned 2.31.10 bill query and response omit the enum. These values
	// match that release's BillStatus; a declared catalog enum takes precedence.
	if len(values) == 0 && op.Path == "/api/bills" {
		values = []string{"draft", "unpaid", "partial", "paid", "void"}
	}
	if len(values) == 0 {
		return &output.Error{Code: "usage", Message: "catalog does not declare status values"}
	}
	for _, valid := range values {
		if value == valid {
			return nil
		}
	}
	return &output.Error{Code: "usage", Message: "invalid --status; valid values: " + strings.Join(values, ", ")}
}
func singular(noun string) string {
	switch noun {
	case "expenses":
		return "expense"
	case "journal":
		return "journal entry"
	}
	return strings.TrimSuffix(noun, "s")
}
