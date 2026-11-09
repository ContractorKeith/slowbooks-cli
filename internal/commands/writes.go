package commands

// Writes describes the supported named mutations and their request field mappings.
func Writes() []Row {
	rows := []Row{
		{Name: "invoices create", Method: "POST", Path: "/api/invoices", Write: true, Flags: []Flag{{"customer", "customer_id"}, {"date", "date"}, {"due", "due_date"}, {"memo", "notes"}}},
		{Name: "payments record", Method: "POST", Path: "/api/payments", Write: true, Flags: []Flag{{"customer", "customer_id"}, {"amount", "amount"}, {"date", "date"}, {"method", "method"}}},
		{Name: "bill-payments record", Method: "POST", Path: "/api/bill-payments", Write: true, Flags: []Flag{{"vendor", "vendor_id"}, {"amount", "amount"}, {"date", "date"}}},
	}
	for _, read := range Reads() {
		switch read.Name {
		case "invoices show", "bills show", "expenses show", "journal show", "payments show", "bill-payments show":
			rows = append(rows, Row{Name: read.Name[:len(read.Name)-4] + "void", Method: "POST", Path: read.Path + "/void", Args: read.Args, Write: true})
		}
	}
	return rows
}
