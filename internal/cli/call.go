package cli

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

func (s *session) callCommand(name string, allowUnknown bool) *cobra.Command {
	var params []string
	var data, key string
	var dry bool
	cmd := &cobra.Command{Use: name + " <METHOD> <path>", Short: "Call a server operation", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(cmd *cobra.Command, args []string) error {
		method := strings.ToUpper(args[0])
		target, err := url.Parse(args[1])
		if err != nil {
			return &output.Error{Code: "usage", Message: "invalid path"}
		}
		// Validate policy before catalog HTTP, including api's unknown-operation escape.
		if target.IsAbs() || target.Host != "" || target.Fragment != "" {
			return &output.Error{Code: "usage", Message: "supply a local /api/ path"}
		}
		cl, cat, err := s.loadCatalog(cmd, dry)
		if err != nil {
			return err
		}
		op, known := cat.Lookup(method, target.Path)
		if !known && !allowUnknown {
			return unknownOperation()
		}
		if !known {
			if err = s.warn("operation missing from catalog; use sbp ops find <words> to discover operations"); err != nil {
				return err
			}
		}
		query := target.Query()
		for _, param := range params {
			k, v, ok := strings.Cut(param, "=")
			if !ok || k == "" {
				return &output.Error{Code: "usage", Message: "--param requires k=v"}
			}
			placeholder := "{" + k + "}"
			if strings.Contains(target.Path, placeholder) {
				target.Path = strings.ReplaceAll(target.Path, placeholder, v)
			} else {
				query.Add(k, v)
			}
		}
		if strings.ContainsAny(target.Path, "{}") {
			return &output.Error{Code: "usage", Message: "missing path parameter", Hint: "supply --param name=value for every path parameter"}
		}
		resolved, _ := cat.Lookup(method, target.Path)
		op.Keyed = op.Keyed || resolved.Keyed
		op.FileRoute = op.FileRoute || resolved.FileRoute

		if op.FileRoute || catalog.IsFile(target.Path) {
			return &output.Error{Code: "usage", Message: "file routes require download", Hint: "sbp download " + target.Path}
		}
		if (op.Keyed || catalog.IsKeyed(method, target.Path)) && method != "GET" && method != "HEAD" && strings.TrimSpace(key) == "" {
			return &output.Error{Code: "usage", Message: "this operation requires --key", Hint: "supply --key <payment-key>; reuse the same key when rerunning"}
		}
		target.RawQuery = query.Encode()
		body, err := readData(cmd, data)
		if err != nil {
			return err
		}
		var value any
		if body != nil {
			value = json.RawMessage(body)
		}
		preview, err := cl.Preview(method, target.String(), value, key)
		if err != nil {
			return err
		}
		if dry {
			return s.w.Success(preview, "Dry run; no request sent", nil)
		}
		response, err := cl.Do(cmd.Context(), method, target.String(), body, key)
		if err != nil {
			return err
		}
		var result any
		if len(response) > 0 && method != "HEAD" {
			if json.Unmarshal(response, &result) != nil {
				return &output.Error{Code: "server", Message: "invalid JSON from server"}
			}
		}
		return s.w.Success(result, method+" "+target.Path, nil)
	}}
	cmd.Flags().StringArrayVar(&params, "param", nil, "Path or query parameter k=v (repeatable)")
	cmd.Flags().StringVar(&data, "data", "", "JSON, @file or - for stdin")
	cmd.Flags().StringVar(&key, "key", "", "Caller-chosen payment key")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "Preview from cached catalog without sending requests")
	return cmd
}
func readData(cmd *cobra.Command, data string) ([]byte, error) {
	if !cmd.Flags().Changed("data") {
		return nil, nil
	}
	return client.ReadData(cmd.InOrStdin(), data)
}
