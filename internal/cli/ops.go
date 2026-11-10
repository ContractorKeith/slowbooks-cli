package cli

import (
	"fmt"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

func (s *session) loadCatalog(cmd *cobra.Command, offline bool) (*client.Client, *catalog.Catalog, error) {
	conn, cl, err := s.connect(cmd)
	if err != nil {
		return nil, nil, err
	}
	cat, err := catalog.Load(cmd.Context(), cl, conn.Profile, offline)
	if err == nil && !offline {
		err = s.warn(versionWarning(cat.Version))
		if err == nil {
			err = s.warn(cat.Warning)
		}
	}
	return cl, cat, err
}
func (s *session) opsCommands() *cobra.Command {
	ops := &cobra.Command{Use: "ops", Short: "Find, describe and call server operations"}
	limit, all := 20, false
	find := &cobra.Command{Use: "find <words...>", Args: usageArgs(cobra.MinimumNArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		words := strings.Fields(strings.Join(args, " "))
		if len(words) == 0 || limit < 1 || (all && cmd.Flags().Changed("limit")) {
			return &output.Error{Code: "usage", Message: "supply search words and a positive --limit or --all"}
		}
		_, cat, err := s.loadCatalog(cmd, false)
		if err != nil {
			return err
		}
		matches := cat.Find(words, len(cat.Operations))
		total := len(matches)
		summary := "Matching operations"
		if !all && total > limit {
			matches = matches[:limit]
			summary = fmt.Sprintf("%d of %d shown; --all for everything", limit, total)
		}
		rows := make([]operationMatch, len(matches))
		for i, op := range matches {
			rows[i] = operationMatch{op.Method, op.Path, op.Summary, op.Tags, op.Keyed, op.FileRoute}
		}
		s.w.Columns = []string{"method", "path", "summary"}
		defer func() { s.w.Columns = nil }()
		return s.w.Success(rows, summary, nil)
	}}
	find.Flags().BoolVar(&all, "all", false, "Show every matching operation")
	find.Flags().IntVar(&limit, "limit", 20, "Maximum operations")
	describe := &cobra.Command{Use: "describe <METHOD> <path>", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(cmd *cobra.Command, args []string) error {
		_, cat, err := s.loadCatalog(cmd, false)
		if err != nil {
			return err
		}
		op, ok := cat.Lookup(args[0], args[1])
		if !ok {
			return unknownOperation()
		}
		return s.w.Success(cat.Describe(op), fmt.Sprintf("%s %s", op.Method, op.Path), nil)
	}}
	ops.AddCommand(find, describe, s.callCommand("call", false))
	return ops
}
func unknownOperation() error {
	return &output.Error{Code: "usage", Message: "unknown operation", Hint: "use slowbooks ops find <words> to find an operation"}
}

// operationMatch leaves parameter and body schemas to ops describe.
type operationMatch struct {
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	Summary   string   `json:"summary"`
	Tags      []string `json:"tags"`
	Keyed     bool     `json:"keyed"`
	FileRoute bool     `json:"file_route"`
}
