package cli

import (
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func (s *session) validateStatus(cat *catalog.Catalog, op catalog.Operation, value string) error {
	values := cat.QueryEnum(op, "status")
	// 2.31.10 omits BillStatus from its query and response schemas.
	if len(values) == 0 && op.Path == "/api/bills" && cat.Version == "2.31.10" {
		values = []string{"draft", "unpaid", "partial", "paid", "void"}
	}
	if len(values) == 0 {
		return s.warn("catalog does not declare status values; passing --status through to the server")
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
	case "journal":
		return "journal entry"
	}
	return strings.TrimSuffix(noun, "s")
}
