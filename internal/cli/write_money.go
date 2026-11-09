package cli

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

var plainMoney = regexp.MustCompile(`^(0|[1-9][0-9]{0,12})(\.[0-9]{1,2})?$`)

// checkMoney refuses inputs the server could round or reinterpret before posting.
func checkMoney(value, field string) error {
	if !plainMoney.MatchString(value) || strings.Trim(value, "0.") == "" {
		return &output.Error{Code: "usage", Message: field + " must be a positive plain decimal with at most 13 integer digits and 2 fractional digits; no sign or exponent"}
	}
	return nil
}

func rawMoney(raw json.RawMessage, field string) error {
	value := string(bytes.TrimSpace(raw))
	if strings.HasPrefix(value, `"`) {
		if json.Unmarshal(raw, &value) != nil {
			value = ""
		}
	}
	return checkMoney(value, field)
}

func checkPaymentMoney(fields map[string]json.RawMessage, idField string) error {
	if raw, ok := fields["amount"]; ok {
		if err := rawMoney(raw, "amount"); err != nil {
			return err
		}
	}
	raw, ok := fields["allocations"]
	if !ok {
		return nil
	}
	var allocations []map[string]json.RawMessage
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &allocations) != nil {
		return &output.Error{Code: "usage", Message: "allocations must be an array of objects"}
	}
	seen := map[string]bool{}
	for _, allocation := range allocations {
		id := string(bytes.TrimSpace(allocation[idField]))
		if strings.HasPrefix(id, `"`) {
			_ = json.Unmarshal(allocation[idField], &id)
		}
		if !integerInput.MatchString(id) {
			return &output.Error{Code: "usage", Message: "allocation " + idField + " must be a positive integer"}
		}
		if seen[id] {
			return &output.Error{Code: "usage", Message: "repeated allocation " + idField + " " + id}
		}
		seen[id] = true
		if err := rawMoney(allocation["amount"], "allocation amount"); err != nil {
			return err
		}
	}
	return nil
}
