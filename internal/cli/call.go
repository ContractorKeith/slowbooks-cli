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
		return s.runCall(cmd, args, callOptions{params: params, data: data, key: key, dry: dry, allowUnknown: allowUnknown, keySet: cmd.Flags().Changed("key"), dataSet: cmd.Flags().Changed("data")})
	}}
	cmd.Flags().StringArrayVar(&params, "param", nil, "Path or query parameter k=v (repeatable)")
	cmd.Flags().StringVar(&data, "data", "", "JSON, @file or - for stdin")
	cmd.Flags().StringVar(&key, "key", "", "Caller-chosen payment key")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "Preview from cached catalog without sending requests")
	return cmd
}

// callOptions keeps named reads on the same validation and request path as ops call.
type callOptions struct {
	params                             []string
	data, key                          string
	dry, allowUnknown, keySet, dataSet bool
	limit                              int
	list, all                          bool
	prepare                            func(*catalog.Catalog, catalog.Operation, url.Values) error
	render                             func(any, bool) error
}

func (s *session) runCall(cmd *cobra.Command, args []string, opts callOptions) error {
	method := strings.ToUpper(args[0])
	if opts.keySet {
		if err := client.ValidateKey(opts.key); err != nil {
			return err
		}
	}
	target, err := url.Parse(args[1])
	if err != nil {
		return &output.Error{Code: "usage", Message: "invalid path"}
	}
	// Validate policy before catalog HTTP, including api's unknown-operation escape.
	if target.IsAbs() || target.Host != "" || target.Fragment != "" {
		return &output.Error{Code: "usage", Message: "supply a local /api/ path"}
	}
	query, err := url.ParseQuery(target.RawQuery)
	if err != nil {
		return &output.Error{Code: "usage", Message: "malformed query string"}
	}
	if method != "GET" && method != "HEAD" && strings.HasSuffix(target.Path, "/") {
		return &output.Error{Code: "usage", Message: "trailing slash on a write is not allowed", Hint: "remove the trailing slash; payment writes require --key"}
	}
	cl, cat, err := s.loadCatalog(cmd, opts.dry)
	if err != nil {
		return err
	}
	op, known := cat.Lookup(method, target.Path)
	if !known && !opts.allowUnknown {
		return unknownOperation()
	}
	if !known {
		if err = s.warn("operation missing from catalog; use sbp ops find <words> to discover operations"); err != nil {
			return err
		}
	}
	for _, param := range opts.params {
		k, v, ok := strings.Cut(param, "=")
		if !ok || k == "" {
			return &output.Error{Code: "usage", Message: "--param requires k=v"}
		}
		placeholder := "{" + k + "}"
		if strings.Contains(target.Path, placeholder) {
			if err := client.ValidatePathParam(v); err != nil {
				return err
			}
			target.Path = strings.ReplaceAll(target.Path, placeholder, v)
			target.RawPath = ""
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
	if len(op.ResponseMedia) == 0 {
		op.ResponseMedia = resolved.ResponseMedia
	}

	if op.FileRoute || ((method == "GET" || method == "HEAD") && catalog.IsFile(target.Path)) {
		return &output.Error{Code: "usage", Message: "file routes require download", Hint: "sbp download " + target.Path}
	}
	keyed := op.Keyed || catalog.IsKeyed(method, target.Path)
	cl = cl.ForOperation(keyed)
	if opts.key != "" && !keyed {
		if err := s.warn("server ignores --key on this unkeyed operation; check the books after an uncertain write"); err != nil {
			return err
		}
	}
	if keyed && method != "GET" && method != "HEAD" && strings.TrimSpace(opts.key) == "" {
		return &output.Error{Code: "usage", Message: "this operation requires --key", Hint: "supply --key <payment-key>; reuse the same key when rerunning"}
	}
	if opts.prepare != nil {
		if err := opts.prepare(cat, op, query); err != nil {
			return err
		}
	}
	target.RawQuery = query.Encode()
	body, err := readCallData(cmd, opts)
	if err != nil {
		return err
	}
	var value any
	if body != nil {
		value = json.RawMessage(body)
	}
	preview, err := cl.Preview(method, target.String(), value, opts.key)
	if err != nil {
		return err
	}
	if opts.dry {
		return s.w.Success(preview, "Dry run; no request sent", nil)
	}

	result, more, err := s.callResult(cmd, cl, method, target, body, opts, op)
	if err != nil {
		return err
	}
	if opts.render != nil {
		return opts.render(result, more)
	}
	if method != "GET" && method != "HEAD" {
		return s.w.SuccessWrite(result, method+" "+target.Path, nil)
	}
	return s.w.Success(result, method+" "+target.Path, nil)
}

func readCallData(cmd *cobra.Command, opts callOptions) ([]byte, error) {
	if !opts.dataSet {
		return nil, nil
	}
	return client.ReadData(cmd.InOrStdin(), opts.data)
}
