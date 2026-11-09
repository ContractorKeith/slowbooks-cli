package fakeserver

import (
	"github.com/ContractorKeith/slowbooks-cli/testdata"
	"net/http"
)

func serveDownload(w http.ResponseWriter, r *http.Request) bool {
	var fixture, media, disposition string
	switch r.URL.Path {
	case "/api/invoices/1/pdf":
		fixture, media, disposition = "invoice.pdf", "application/pdf", `attachment; filename="invoice-1.pdf"`
	case "/api/invoices/traversal/pdf":
		fixture, media, disposition = "invoice.pdf", "application/pdf", `attachment; filename="../../evil.pdf"`
	case "/api/csv/export/customers":
		fixture, media, disposition = "customers.csv", "text/csv", `attachment; filename="customers.csv"`
	default:
		return false
	}
	w.Header().Set("Content-Type", media)
	w.Header().Set("Content-Disposition", disposition)
	data, _ := testdata.Files.ReadFile("fake/" + fixture)
	_, _ = w.Write(data)
	return true
}
