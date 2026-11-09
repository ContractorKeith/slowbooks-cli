package cli

import (
	"io"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

// Build identifies this binary; release builds inject these values through main.
type Build struct{ Version, Commit string }

// NewRoot constructs an isolated command tree for each invocation.
func NewRoot(w *output.Writer, build Build) *cobra.Command {
	var profile, company, jq string
	var timeout int
	var noColor bool
	root := &cobra.Command{
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		Use:               "sbp", Short: "SlowBooks Pro from the shell", SilenceErrors: true, SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if jq != "" {
				return &output.Error{Code: "usage", Message: "--jq: not yet"}
			}
			if timeout < 1 || timeout > 3600 {
				return &output.Error{Code: "usage", Message: "--timeout must be 1–3600 seconds"}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return &output.Error{Code: "usage", Message: "a command is required; see sbp --help"}
		},
	}
	root.SetOut(w.HelpWriter())
	root.SetErr(w.HelpWriter())
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error { return &output.Error{Code: "usage", Message: err.Error()} })
	flags := root.PersistentFlags()
	flags.StringVar(&profile, "profile", "", "Saved profile")
	flags.StringVar(&company, "company", "", "Company")
	flags.BoolVar(&w.JSON, "json", false, "Print JSON")
	flags.BoolVar(&w.Quiet, "quiet", false, "Print data only")
	flags.StringVar(&jq, "jq", "", "Filter JSON (not yet)")
	flags.IntVar(&timeout, "timeout", 120, "Request timeout in seconds")
	flags.BoolVar(&noColor, "no-color", false, "Disable color")
	root.AddCommand(&cobra.Command{
		Use: "version", Short: "Show build and verified server versions", Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return &output.Error{Code: "usage", Message: err.Error()}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return w.Success(struct {
				Version  string   `json:"version"`
				Commit   string   `json:"commit"`
				Verified []string `json:"verified_sbp"`
			}{build.Version, build.Commit, commands.VerifiedVersions()}, "sbp "+build.Version, nil)
		},
	})
	return root
}

// Execute renders all errors exactly once, including cobra parse errors.
func Execute(root *cobra.Command, w *output.Writer, args []string) int {
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if strings.HasPrefix(err.Error(), "unknown command ") {
			err = &output.Error{Code: "usage", Message: err.Error()}
		}
		return w.Failure(err)
	}
	return output.ExitCode(nil)
}

// Run is the process boundary; tests can execute the same root in process.
func Run(args []string, out, err io.Writer, build Build) int {
	w := output.New(out, err)
	return Execute(NewRoot(w, build), w, args)
}
