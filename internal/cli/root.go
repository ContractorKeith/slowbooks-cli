package cli

import (
	"io"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/config"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

// Build identifies this binary; release builds inject these values through main.
type Build struct{ Version, Commit string }

// NewRoot constructs an isolated command tree for each invocation.
func NewRoot(w *output.Writer, build Build) *cobra.Command {
	s := &session{w: w, build: build}
	var jq string
	var noColor bool
	jsonOutput, quiet := w.JSON, w.Quiet
	root := &cobra.Command{
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		Use:               "sbp", Short: "SlowBooks Pro from the shell", SilenceErrors: true, SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			w.JSON, w.Quiet = jsonOutput, quiet
			if err := w.SetJQ(jq, cmd.Flags().Changed("jq")); err != nil {
				return err
			}
			s.flags.ProfileSet = cmd.Flags().Changed("profile")
			s.flags.CompanySet = cmd.Flags().Changed("company")
			s.flags.TimeoutSet = cmd.Flags().Changed("timeout")
			s.flags.URLSet = cmd.Flags().Changed("url")
			timeout, e := config.Timeout(s.flags)
			if e != nil {
				return e
			}
			w.SetContext(cmd.Context(), timeout)
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
	flags.StringVar(&s.flags.Profile, "profile", "", "Saved profile")
	flags.StringVar(&s.flags.Company, "company", "", "Company")
	flags.BoolVar(&jsonOutput, "json", jsonOutput, "Print JSON")
	flags.BoolVar(&quiet, "quiet", quiet, "Print data only")
	flags.StringVar(&jq, "jq", "", "Filter JSON with jq")
	flags.IntVar(&s.flags.Timeout, "timeout", 120, "Request timeout in seconds")
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
	root.AddCommand(s.authCommands(), s.profileCommands(), s.doctorCommand(), s.opsCommands(), s.callCommand("api", true))
	s.addReads(root)
	return root
}

// Execute renders all errors exactly once, including cobra parse errors.
func Execute(root *cobra.Command, w *output.Writer, args []string) int {
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if rendered, ok := err.(*checksFailed); ok {
			return output.ExitCode(rendered.cause)
		}
		// Argument and flag errors can happen before PersistentPreRunE.
		w.JSON, _ = root.PersistentFlags().GetBool("json")
		w.Quiet, _ = root.PersistentFlags().GetBool("quiet")
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
