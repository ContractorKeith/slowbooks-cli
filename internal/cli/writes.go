package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

func (s *session) addWrites(root *cobra.Command) {
	for _, row := range commands.Writes() {
		parts := strings.Fields(row.Name)
		for _, parent := range root.Commands() {
			if parent.Name() == parts[0] {
				parent.AddCommand(s.writeCommand(row, parts[1]))
				break
			}
		}
	}
}

func (s *session) writeCommand(row commands.Row, name string) *cobra.Command {
	var data, key string
	var dry, fee bool
	var applies []string
	values := make([]string, len(row.Flags))
	use := name
	if len(row.Args) > 0 {
		use += " <id>"
	}
	cmd := &cobra.Command{Use: use, Short: row.Name, Args: usageArgs(cobra.ExactArgs(len(row.Args)))}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if name == "record" && key == "" {
			return &output.Error{Code: "usage", Message: "this operation requires --key", Hint: "supply a stable --key such as a check number or bank reference; reuse it when rerunning"}
		}
		opts := callOptions{key: key, keySet: cmd.Flags().Changed("key"), dry: dry}
		path := row.Path
		if len(args) > 0 {
			if err := client.ValidatePathParam(args[0]); err != nil {
				return err
			}
			path = strings.ReplaceAll(path, "{"+row.Args[0]+"}", url.PathEscape(args[0]))
		}
		if fee {
			opts.params = []string{"fee_refunded=true"}
		}
		body, err := writeData(cmd, row, values, data, applies)
		if err != nil {
			return err
		}
		if body != nil {
			opts.data = string(body)
			opts.dataSet = true
		}
		kind := strings.Fields(row.Name)[0]
		opts.render = func(value any, _ bool) error {
			next := "sbp " + kind + " list"
			if obj, ok := value.(map[string]any); ok && obj["id"] != nil {
				next = fmt.Sprintf("sbp %s show %v", kind, obj["id"])
			}
			return s.w.Success(value, row.Name, []string{next})
		}
		err = s.runCall(cmd, []string{row.Method, path}, opts)
		if err != nil && name != "record" {
			e := output.AsError(err)
			if e.Code == "network" {
				e.Hint = "check sbp " + kind + " list --limit 5 before running this write again"
			}
			return e
		}
		return err
	}
	cmd.Flags().StringVar(&data, "data", "", "JSON, @file or - for stdin; flags override fields")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "Preview without sending requests")
	for i, flag := range row.Flags {
		cmd.Flags().StringVar(&values[i], flag.Name, "", "Set "+flag.Param)
	}
	if name == "record" {
		cmd.Flags().StringVar(&key, "key", "", "Stable check number or bank reference")
		cmd.Flags().StringArrayVar(&applies, "apply", nil, "Allocation ID=AMOUNT (repeatable)")
	}
	if row.Name == "payments void" {
		cmd.Flags().BoolVar(&fee, "fee-refunded", false, "The payment fee was refunded")
	}
	return cmd
}

var decimalInput = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
var integerInput = regexp.MustCompile(`^[1-9][0-9]*$`)

// writeData merges raw JSON fields so amounts and large IDs never pass through floats.
func writeData(cmd *cobra.Command, row commands.Row, values []string, data string, applies []string) ([]byte, error) {
	fields := map[string]json.RawMessage{}
	if cmd.Flags().Changed("data") {
		raw, err := client.ReadData(cmd.InOrStdin(), data)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &fields); err != nil || fields == nil {
			return nil, &output.Error{Code: "usage", Message: "--data must be a JSON object"}
		}
	}
	for i, flag := range row.Flags {
		if !cmd.Flags().Changed(flag.Name) {
			continue
		}
		value := values[i]
		var raw []byte
		if strings.HasSuffix(flag.Param, "_id") || flag.Param == "amount" {
			valid := decimalInput.MatchString(value)
			if strings.HasSuffix(flag.Param, "_id") {
				valid = integerInput.MatchString(value)
			}
			if !valid {
				return nil, &output.Error{Code: "usage", Message: "invalid --" + flag.Name}
			}
			raw = []byte(value)
		} else {
			raw, _ = json.Marshal(value)
		}
		fields[flag.Param] = raw
	}
	if cmd.Flags().Changed("apply") {
		allocations := []map[string]json.RawMessage{}
		idField := "invoice_id"
		if strings.HasPrefix(row.Name, "bill-payments") {
			idField = "bill_id"
		}
		for _, apply := range applies {
			id, amount, ok := strings.Cut(apply, "=")
			if !ok || !integerInput.MatchString(id) || !decimalInput.MatchString(amount) {
				return nil, &output.Error{Code: "usage", Message: "--apply requires ID=AMOUNT"}
			}
			allocations = append(allocations, map[string]json.RawMessage{idField: json.RawMessage(id), "amount": json.RawMessage(amount)})
		}
		fields["allocations"], _ = json.Marshal(allocations)
	}
	if len(row.Flags) > 0 {
		required := []string{"date"}
		if strings.HasPrefix(row.Name, "bill-payments") {
			required = append(required, "vendor_id", "amount")
		} else {
			required = append(required, "customer_id")
			if strings.HasSuffix(row.Name, "record") {
				required = append(required, "amount")
			}
		}
		for _, field := range required {
			if len(fields[field]) == 0 || bytes.Equal(bytes.TrimSpace(fields[field]), []byte("null")) {
				return nil, &output.Error{Code: "usage", Message: "missing required field " + field, Hint: "supply its flag or --data field"}
			}
		}
	}
	if len(fields) == 0 && !cmd.Flags().Changed("data") {
		return nil, nil
	}
	return json.Marshal(fields)
}
