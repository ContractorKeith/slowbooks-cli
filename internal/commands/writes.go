package commands

import "strings"

// Writes describes the supported named mutations and their request field mappings.
func Writes() []Row {
	rows := []Row{
		{Name: "invoices create", Method: "POST", Path: "/api/invoices", Write: true, Required: []string{"customer_id", "date"}, Flags: []Flag{{"customer", "customer_id"}, {"date", "date"}, {"due", "due_date"}, {"memo", "notes"}}},
		{Name: "payments record", Method: "POST", Path: "/api/payments", Write: true, Required: []string{"customer_id", "date", "amount"}, AllocationID: "invoice_id", Flags: []Flag{{"customer", "customer_id"}, {"amount", "amount"}, {"date", "date"}, {"method", "method"}}},
		{Name: "bill-payments record", Method: "POST", Path: "/api/bill-payments", Write: true, Required: []string{"vendor_id", "date", "amount"}, AllocationID: "bill_id", Flags: []Flag{{"vendor", "vendor_id"}, {"amount", "amount"}, {"date", "date"}}},
	}
	for _, read := range Reads() {
		switch read.Name {
		case "invoices show", "bills show", "expenses show", "journal show", "payments show", "bill-payments show":
			rows = append(rows, Row{Name: read.Name[:len(read.Name)-4] + "void", Method: "POST", Path: read.Path + "/void", Args: read.Args, Write: true})
		}
	}
	for i := range rows {
		rows[i].Next = []string{"sbp " + strings.Fields(rows[i].Name)[0] + " show <id>"}
	}
	return rows
}
