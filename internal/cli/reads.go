package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

func (s *session) addReads(root *cobra.Command) {
	groups := map[string]*cobra.Command{}
	for _, row := range commands.Reads() {
		parts := strings.Fields(row.Name)
		parent := root
		if len(parts) == 2 {
			parent = groups[parts[0]]
			if parent == nil {
				parent = &cobra.Command{Use: parts[0], Short: "Read " + parts[0]}
				groups[parts[0]] = parent
				root.AddCommand(parent)
			}
		}
		parent.AddCommand(s.readCommand(row, parts[len(parts)-1]))
	}
}

func (s *session) readCommand(row commands.Row, name string) *cobra.Command {
	limit, all := 50, false
	values := make([]string, len(row.Flags))
	var params []string
	use := name
	for _, arg := range row.Args {
		label := arg
		if name == "show" {
			label = "id"
		}
		if row.Name == "search" {
			label = "query"
		}
		use += " <" + label + ">"
	}
	cmd := &cobra.Command{Use: use, Short: row.Name, Args: usageArgs(cobra.ExactArgs(len(row.Args)))}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if row.List && (limit < 1 || (all && cmd.Flags().Changed("limit"))) {
			return &output.Error{Code: "usage", Message: "use a positive --limit or --all"}
		}
		opts := callOptions{params: append([]string{}, params...)}
		path := row.Path
		for i, arg := range row.Args {
			if strings.Contains(path, "{"+arg+"}") {
				if err := client.ValidatePathParam(args[i]); err != nil {
					return err
				}
				path = strings.ReplaceAll(path, "{"+arg+"}", url.PathEscape(args[i]))
			} else {
				opts.params = append(opts.params, arg+"="+args[i])
			}
		}
		for i, flag := range row.Flags {
			if cmd.Flags().Changed(flag.Name) {
				opts.params = append(opts.params, flag.Param+"="+values[i])
			}
		}
		if row.List && !all {
			opts.limit = limit
		}
		opts.render = func(data any) error {
			summary := row.Name
			if row.List {
				if list, ok := data.([]any); ok {
					total := len(list)
					summary = fmt.Sprintf("%d %s", total, row.Noun)
					if !all && total > limit {
						data = list[:limit]
						summary = fmt.Sprintf("%d of %d shown; --all for everything", limit, total)
					}
				}
			}
			s.w.Columns = row.Columns
			return s.w.Success(data, summary, row.Next)
		}
		if row.Catalog {
			_, cat, err := s.loadCatalog(cmd, false)
			if err != nil {
				return err
			}
			reports := commands.Reports(cat)
			data := make([]any, len(reports))
			for i, r := range reports {
				data[i] = r
			}
			return opts.render(data)
		}
		return s.runCall(cmd, []string{row.Method, path}, opts)
	}
	if row.List {
		cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows")
		cmd.Flags().BoolVar(&all, "all", false, "Show every row")
	}
	for i, flag := range row.Flags {
		cmd.Flags().StringVar(&values[i], flag.Name, "", "Filter by "+flag.Name)
	}
	if row.Name == "reports run" {
		cmd.Flags().StringArrayVar(&params, "param", nil, "Report query parameter k=v (repeatable)")
	}
	return cmd
}
