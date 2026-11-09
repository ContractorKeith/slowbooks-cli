package commands

import (
	"fmt"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
)

func checkWriteSchema(cat *catalog.Catalog, row Row) error {
	if row.Name == "payments void" {
		op, _ := cat.Lookup(row.Method, row.Path)
		found := false
		for _, p := range op.Params {
			if p.Name == "fee_refunded" && p.In == "query" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("payments void: missing fee_refunded query")
		}
	}
	if !row.Write || len(row.Flags) == 0 {
		return nil
	}
	op, _ := cat.Lookup(row.Method, row.Path)
	props := schemaProperties(cat.Document, op.Body, 0)
	for _, flag := range row.Flags {
		if _, ok := props[flag.Param]; !ok {
			return fmt.Errorf("%s: missing body field %s", row.Name, flag.Param)
		}
	}
	if row.Name == "payments record" || row.Name == "bill-payments record" {
		id := "invoice_id"
		if row.Name == "bill-payments record" {
			id = "bill_id"
		}
		alloc := schemaProperties(cat.Document, props["allocations"], 0)
		for _, field := range []string{id, "amount"} {
			if _, ok := alloc[field]; !ok {
				return fmt.Errorf("%s: missing allocation %s", row.Name, field)
			}
		}
		// Keyed comes from the hard-coded local list: this snapshot declares no
		// Idempotency-Key header. This checks local policy; e2e replay is the
		// real verification that the server deduplicates the payment.
		if !op.Keyed {
			return fmt.Errorf("%s: missing key contract", row.Name)
		}
	}
	return nil
}
