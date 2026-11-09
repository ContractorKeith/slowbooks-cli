package commands

import (
	"fmt"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
)

func checkColumns(cat *catalog.Catalog, row Row) error {
	if len(row.Columns) == 0 {
		return nil
	}
	paths, _ := cat.Document["paths"].(map[string]any)
	item, _ := paths[row.Path].(map[string]any)
	op, _ := item[strings.ToLower(row.Method)].(map[string]any)
	responses, _ := op["responses"].(map[string]any)
	response, _ := responses["200"].(map[string]any)
	content, _ := response["content"].(map[string]any)
	media, _ := content["application/json"].(map[string]any)
	props := schemaProperties(cat.Document, media["schema"], 0)
	for _, column := range row.Columns {
		if _, ok := props[column]; !ok {
			return fmt.Errorf("%s: column %s absent from 200 response schema", row.Name, column)
		}
	}
	return nil
}

func schemaProperties(doc map[string]any, raw any, depth int) map[string]any {
	if depth > 16 {
		return nil
	}
	schema, _ := raw.(map[string]any)
	if ref, ok := schema["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
		var resolved any = doc
		for _, key := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			obj, _ := resolved.(map[string]any)
			resolved = obj[key]
		}
		return schemaProperties(doc, resolved, depth+1)
	}
	if items, ok := schema["items"]; ok {
		return schemaProperties(doc, items, depth+1)
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
	}
	if all, ok := schema["allOf"].([]any); ok {
		for _, part := range all {
			for k, v := range schemaProperties(doc, part, depth+1) {
				props[k] = v
			}
		}
	}
	return props
}

func checkReportMappings(cat *catalog.Catalog) error {
	// Pin the date contract for every mapped report in the verified snapshot.
	for _, group := range []struct{ names, from, to string }{
		{"account-transactions adjusting-entries cash-flow closing-date-exceptions functional-expenses fund-balances general-ledger income-by-customer job-cost-by-vendor job-profitability pledges possible-duplicates profit-loss profit-loss-by-class profit-loss-by-job profit-loss-detail purchases-by-item sales-by-item sales-tax statement-of-activities trial-balance", "start_date", "end_date"},
		{"ap-aging ar-aging balance-sheet statement-of-financial-position", "", "as_of_date"},
		{"buyout change-order-log commitment-status cost-to-complete cost-to-complete-forecast wip-schedule", "", "as_of"},
	} {
		for _, name := range strings.Fields(group.names) {
			op, ok := cat.Lookup("GET", "/api/reports/"+name)
			if !ok {
				return fmt.Errorf("missing mapped report %s", name)
			}
			if ReportParam(op, "from") != group.from || ReportParam(op, "to") != group.to {
				return fmt.Errorf("%s report date mapping drift", name)
			}
		}
	}
	for _, report := range Reports(cat) {
		op, _ := cat.Lookup("GET", "/api/reports/"+report.Name)
		for _, flag := range []string{"from", "to"} {
			target := ReportParam(op, flag)
			if target == "" {
				if report.Name == "profit-loss" {
					return fmt.Errorf("missing required report date --%s", flag)
				}
				continue
			}
			declared := false
			for _, p := range op.Params {
				if p.In == "query" && p.Name == target {
					declared = true
				}
			}
			if !declared {
				return fmt.Errorf("%s report date --%s maps to undeclared %s", report.Name, flag, target)
			}
		}
	}
	return nil
}
