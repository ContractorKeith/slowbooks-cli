package cli

import (
	"fmt"
	"strings"
)

// voidSummary prefers the document number while show hints retain the route ID.
func voidSummary(kind, id string, value any) string {
	noun := strings.ReplaceAll(singular(kind), "-", " ")
	if obj, ok := value.(map[string]any); ok {
		for _, field := range []string{"invoice_number", "bill_number", "expense_number", "entry_number", "payment_number", "number"} {
			if number, ok := obj[field]; ok && number != nil && fmt.Sprint(number) != "" {
				return fmt.Sprintf("Voided %s %v", noun, number)
			}
		}
	}
	return "Voided " + noun + " " + id
}
