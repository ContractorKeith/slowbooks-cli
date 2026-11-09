package commands

import "github.com/ContractorKeith/slowbooks-cli/internal/catalog"

// ReportParam maps date flags only to query parameters declared by this report.
func ReportParam(op catalog.Operation, flag string) string {
	candidates := []string{"start_date"}
	if flag == "to" {
		candidates = []string{"end_date", "as_of_date", "as_of"}
	}
	for _, name := range candidates {
		for _, p := range op.Params {
			if p.In == "query" && p.Name == name {
				return name
			}
		}
	}
	return ""
}
