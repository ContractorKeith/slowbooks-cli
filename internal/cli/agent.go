package cli

import (
	"encoding/json"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type agentFlag struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Default  string `json:"default"`
	Required bool   `json:"required"`
}
type agentCommand struct {
	Runnable       bool                        `json:"runnable"`
	Name           string                      `json:"name"`
	Args           string                      `json:"args"`
	Flags          []agentFlag                 `json:"flags"`
	Operation      *commands.ManifestOperation `json:"operation,omitempty"`
	Access         string                      `json:"access"`
	Keyed          bool                        `json:"keyed"`
	FileOutput     bool                        `json:"file_output"`
	RequiredFields []string                    `json:"required_fields,omitempty"`
	Examples       []string                    `json:"examples"`
	Next           []string                    `json:"next"`
}
type agentHelp struct {
	Commands  []agentCommand      `json:"commands"`
	ExitCodes []output.ExitStatus `json:"exit_codes"`
}

// describeTree derives the offline agent contract from the executable tree.
func describeTree(root *cobra.Command) agentHelp {
	rows := map[string]commands.Row{}
	for _, row := range append(commands.Reads(), commands.Writes()...) {
		rows["sbp "+row.Name] = row
	}
	result := agentHelp{Commands: []agentCommand{}, ExitCodes: output.ExitCodes()}
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		row, named := rows[cmd.CommandPath()]
		args := strings.TrimSpace(strings.TrimPrefix(cmd.Use, cmd.Name()))
		item := agentCommand{Runnable: cmd.Runnable(), Name: cmd.CommandPath(), Args: args, Flags: []agentFlag{}, Access: "read", Next: []string{}, Examples: []string{cmd.CommandPath()}}
		if args != "" {
			item.Examples[0] += " " + args
		}
		if named {
			item.Keyed = catalog.IsKeyed(row.Method, row.Path)
			item.FileOutput, item.RequiredFields = row.Download, row.Required
			item.Next = append([]string{}, row.Next...)
			if row.Path != "" {
				item.Operation = &commands.ManifestOperation{Method: row.Method, Path: row.Path}
			}
			if row.Write {
				item.Access = "write"
			}
		}
		switch cmd.CommandPath() {
		case "sbp api", "sbp ops call":
			item.Access = "dynamic"
		case "sbp download":
			item.FileOutput = true
		case "sbp auth login", "sbp auth logout", "sbp profile use":
			item.Access = "write"
		}
		referenceFlags(cmd).VisitAll(func(f *pflag.Flag) {
			required := len(f.Annotations[cobra.BashCompOneRequiredFlag]) > 0 || (item.Keyed && f.Name == "key")
			item.Flags = append(item.Flags, agentFlag{f.Name, f.Value.Type(), f.DefValue, required})
			if required {
				item.Examples[0] += " --" + f.Name + " <" + f.Name + ">"
			}
		})
		if row.Write {
			for _, field := range row.Required {
				for _, flag := range row.Flags {
					if flag.Param == field {
						item.Examples[0] += " --" + flag.Name + " <" + flag.Name + ">"
					}
				}
			}
			item.Examples[0] += " --dry-run"
		}
		if !cmd.Runnable() || cmd == cmd.Root() {
			item.Examples[0] += " --help"
		}
		result.Commands = append(result.Commands, item)
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	return result
}

func (s *session) addAgent(root *cobra.Command) {
	var agent, manifest bool
	var checksums string
	root.PersistentFlags().BoolVar(&agent, "agent", false, "Machine-readable help (with --help)")
	cmd := &cobra.Command{Use: "commands", Short: "Describe commands or emit a marketplace manifest", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, args []string) error {
		if manifest {
			value, err := commands.Manifest(s.build.Version, checksums)
			if err != nil {
				return &output.Error{Code: "usage", Message: err.Error()}
			}
			raw, err := json.Marshal(value)
			if err != nil {
				return err
			}
			return s.w.Text(string(raw) + "\n")
		}
		if checksums != "" {
			return &output.Error{Code: "usage", Message: "--checksums requires --manifest"}
		}
		return s.w.Success(describeTree(root), "Command reference", nil)
	}}
	cmd.Flags().BoolVar(&manifest, "manifest", false, "Print marketplace plugin.json without an envelope")
	cmd.Flags().StringVar(&checksums, "checksums", "", "SHA256SUMS for release archives")
	root.AddCommand(cmd)
	root.InitDefaultHelpCmd()
	normalHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if agent {
			help, _ := cmd.Flags().GetBool("help")
			if !help {
				s.w.HelpError = agentUsage()
				return
			}
			s.w.JSON = true
			s.w.HelpError = s.w.Success(describeTree(cmd), "Command reference", nil)
			return
		}
		normalHelp(cmd, args)
	})
}

// referenceFlags combines flag views without copying inherited flags into commands.
func referenceFlags(cmd *cobra.Command) *pflag.FlagSet {
	flags := pflag.NewFlagSet(cmd.CommandPath(), pflag.ContinueOnError)
	flags.AddFlagSet(cmd.Flags())
	for current := cmd; current != nil; current = current.Parent() {
		flags.AddFlagSet(current.PersistentFlags())
	}
	return flags
}

func agentUsage() error { return &output.Error{Code: "usage", Message: "--agent requires --help"} }

// Initialize Cobra's lazy help flags once, while constructing the tree.
func initializeHelp(root *cobra.Command) {
	root.InitDefaultHelpFlag()
	for _, child := range root.Commands() {
		initializeHelp(child)
	}
}
