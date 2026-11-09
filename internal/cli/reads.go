package cli

import (
	"fmt"
	"math"
	"net/url"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
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
		if row.Download {
			parent.AddCommand(s.readDownloadCommand(row))
		} else {
			parent.AddCommand(s.readCommand(row, parts[len(parts)-1]))
		}
	}
}

func (s *session) readCommand(row commands.Row, name string) *cobra.Command {
	limit, all := 50, false
	var open bool
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
		if row.List && (limit < 1 || limit == math.MaxInt || (all && cmd.Flags().Changed("limit"))) {
			return &output.Error{Code: "usage", Message: "use a positive --limit or --all"}
		}
		opts := callOptions{params: append([]string{}, params...), list: row.List, all: all, limit: limit}
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
		opts.prepare = func(cat *catalog.Catalog, op catalog.Operation, query url.Values) error {
			for i, flag := range row.Flags {
				if !cmd.Flags().Changed(flag.Name) {
					continue
				}
				param := flag.Param
				if row.Name == "reports run" {
					param = commands.ReportParam(op, flag.Name)
				}
				if param == "" {
					return undeclaredParam(op, "--"+flag.Name)
				}
				value := values[i]
				if flag.Name == "open" {
					value = fmt.Sprint(open)
				}
				if flag.Name == "status" {
					if err := s.validateStatus(cat, op, value); err != nil {
						return err
					}
				}
				query.Set(param, value)
			}
			for name := range query {
				found := false
				for _, p := range op.Params {
					if p.In == "query" && p.Name == name {
						found = true
					}
				}
				if !found {
					return undeclaredParam(op, name)
				}
			}
			return nil
		}
		opts.render = func(data any, serverMore bool) error {
			summary := row.Name
			if row.Name == "search" {
				summary = "Search results for " + args[0]
			}
			if row.List {
				if list, ok := data.([]any); ok {
					total := len(list)
					noun := row.Noun
					if total == 1 {
						noun = singular(noun)
					}
					summary = fmt.Sprintf("%d %s", total, noun)
					if !all && total > limit {
						data = list[:limit]
						summary = fmt.Sprintf("%d of %d shown; --all for everything", limit, total)
						if serverMore {
							summary = fmt.Sprintf("%d of %d+ shown; --all for everything", limit, limit)
						}
					}
				}
			}
			s.w.Columns = row.Columns
			defer func() { s.w.Columns = nil }()
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
			return opts.render(data, false)
		}
		return s.runCall(cmd, []string{row.Method, path}, opts)
	}
	if row.List {
		cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows")
		cmd.Flags().BoolVar(&all, "all", false, "Show every row")
	}
	for i, flag := range row.Flags {
		if flag.Name == "open" {
			cmd.Flags().BoolVar(&open, "open", false, "Show open records")
			continue
		}
		cmd.Flags().StringVar(&values[i], flag.Name, "", "Filter by "+flag.Name)
	}
	if row.Name == "reports run" {
		cmd.Flags().StringArrayVar(&params, "param", nil, "Report query parameter k=v (repeatable)")
	}
	return cmd
}

func undeclaredParam(op catalog.Operation, name string) error {
	names := []string{}
	for _, p := range op.Params {
		if p.In == "query" {
			names = append(names, p.Name)
		}
	}
	return &output.Error{Code: "usage", Message: fmt.Sprintf("%s has no target on %s; declared query parameters: %s", name, op.Path, strings.Join(names, ", "))}
}
