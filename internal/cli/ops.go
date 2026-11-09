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
	limit := 20
	find := &cobra.Command{Use: "find <words...>", Args: usageArgs(cobra.MinimumNArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		words := strings.Fields(strings.Join(args, " "))
		if len(words) == 0 || limit < 1 {
			return &output.Error{Code: "usage", Message: "supply search words and a positive --limit"}
		}
		_, cat, err := s.loadCatalog(cmd, false)
		if err != nil {
			return err
		}
		return s.w.Success(cat.Find(words, limit), "Matching operations", nil)
	}}
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
