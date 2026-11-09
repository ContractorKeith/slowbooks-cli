package commands

import (
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
)

// Flag maps a friendly flag to a server query parameter or write body field.
type Flag struct{ Name, Param string }

// Row is the shared description of a named operation. Catalog rows are local reads.
type Row struct {
	Name, Method, Path, Noun       string
	Args                           []string
	Required                       []string
	AllocationID                   string
	Flags                          []Flag
	Next, Columns                  []string
	List, Write, Catalog, Download bool
}

// Reads returns independent descriptions; callers may build their own command tree.
func Reads() []Row {
	var rows []Row
	for _, kind := range []struct {
		name, id string
		columns  []string
	}{
		{"customers", "customer_id", []string{"id", "name", "email"}},
		{"vendors", "vendor_id", []string{"id", "name", "email"}},
		{"items", "item_id", []string{"id", "name", "item_type"}},
		{"accounts", "account_id", []string{"id", "name", "account_type"}},
		{"invoices", "invoice_id", []string{"id", "invoice_number", "date", "status", "total"}},
		{"bills", "bill_id", []string{"id", "bill_number", "date", "status", "total"}},
		{"expenses", "expense_id", []string{"id", "date", "amount"}},
		{"payments", "payment_id", []string{"id", "date", "amount"}},
		{"bill-payments", "bill_payment_id", []string{"id", "date", "amount"}},
		{"journal", "entry_id", []string{"id", "date", "description"}},
	} {
		list := Row{Name: kind.name + " list", Method: "GET", Path: "/api/" + kind.name, Noun: kind.name, List: true, Columns: kind.columns}
		if kind.name == "invoices" || kind.name == "bills" {
			list.Flags = []Flag{{"status", "status"}, {"open", "open_only"}}
		}
		rows = append(rows, list, Row{Name: kind.name + " show", Method: "GET", Path: "/api/" + kind.name + "/{" + kind.id + "}", Args: []string{kind.id}, Next: []string{"slowbooks " + kind.name + " list"}})
	}
	return append(rows,
		Row{Name: "reports list", Method: "GET", Catalog: true, List: true, Noun: "reports", Columns: []string{"name", "summary"}},
		Row{Name: "reports run", Method: "GET", Path: "/api/reports/{name}", Args: []string{"name"}, Flags: []Flag{{Name: "from"}, {Name: "to"}}, Next: []string{"slowbooks reports list"}},
		Row{Name: "invoices pdf", Method: "GET", Path: "/api/invoices/{invoice_id}/pdf", Args: []string{"invoice_id"}, Download: true},
		Row{Name: "search", Method: "GET", Path: "/api/search", Args: []string{"q"}, Noun: "results"},
	)
}

type Report struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// Reports discovers direct JSON report operations, excluding file routes and helpers.
func Reports(cat *catalog.Catalog) []Report {
	reports := []Report{}
	for _, op := range cat.Operations {
		name := strings.TrimPrefix(op.Path, "/api/reports/")
		if op.Method == "GET" && strings.HasPrefix(op.Path, "/api/reports/") && !strings.ContainsAny(name, "/{") && !op.FileRoute && name != "first-date" {
			reports = append(reports, Report{name, op.Summary})
		}
	}
	return reports
}
