package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/config"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

type session struct {
	w               *output.Writer
	build           Build
	flags           config.Flags
	companiesLoaded bool
	companies       []map[string]any
	companiesErr    error
}
type status struct {
	URL           string `json:"url"`
	Company       string `json:"company"`
	Role          string `json:"role"`
	ServerVersion string `json:"server_version"`
	TokenFile     string `json:"token_file"`
}

func (s *session) connect(cmd *cobra.Command) (config.Connection, *client.Client, error) {
	c, e := config.Resolve(s.flags)
	if e != nil {
		return c, nil, e
	}
	if e := s.checkEnvTokenURL(c); e != nil {
		return c, nil, e
	}
	token, e := config.TokenFile(c.TokenFile)
	if e != nil {
		return c, nil, e
	}
	cl, e := client.New(cmd.Context(), client.Options{URL: c.URL, Token: token, Company: c.Company, Timeout: c.Timeout, Version: s.build.Version})
	if e != nil {
		return c, nil, e
	}
	offline, _ := cmd.Flags().GetBool("dry-run")
	// Doctor owns its diagnostic row; offline previews must never probe.
	if !offline && cmd.Name() != "doctor" {
		if e := s.checkCompany(cmd, cl, false); e != nil {
			return c, nil, e
		}
	}
	return c, cl, nil
}
func getJSON(cmd *cobra.Command, c *client.Client, path string, dest any) error {
	b, e := c.Do(cmd.Context(), "GET", path, nil, "")
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, dest); e != nil {
		return &output.Error{Code: "error", Message: "invalid JSON from server"}
	}
	return nil
}
func role(cmd *cobra.Command, c *client.Client) (string, error) {
	c = c.WithoutCompany()
	for i, path := range []string{"/api/audit?limit=1", "/api/employees"} {
		_, e := c.Do(cmd.Context(), "GET", path, nil, "")
		if e != nil {
			if output.AsError(e).Code == "refused" {
				return []string{"readonly", "accountant"}[i], nil
			}
			return "unknown", e
		}
	}
	return "bookkeeper", nil
}
func versionWarning(v string) string {
	var major, minor int
	if _, e := fmt.Sscanf(v, "%d.%d", &major, &minor); e != nil {
		return "server version is unknown or unverified: " + v
	}
	if major < 2 || (major == 2 && minor < 31) {
		return "server version " + v + " is below 2.31"
	}
	for _, verified := range commands.VerifiedVersions() {
		if v == verified {
			return ""
		}
	}
	return "server version " + v + " is not in verified_sbp"
}
func (s *session) warn(message string) error {
	if message == "" {
		return nil
	}
	return s.w.Warning(message)
}
func (s *session) inspect(cmd *cobra.Command, c *client.Client) (string, string, error) {
	var system struct {
		Version string `json:"version"`
	}
	if e := getJSON(cmd, c, "/api/system", &system); e != nil {
		return "", "", e
	}
	if e := s.warn(versionWarning(system.Version)); e != nil {
		return "", "", e
	}
	r, e := role(cmd, c)
	if e != nil {
		if warningErr := s.warn("could not determine token role; role is unknown"); warningErr != nil {
			return "", "", warningErr
		}
	}
	return system.Version, r, nil
}
func usageArgs(check cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if e := check(cmd, args); e != nil {
			return &output.Error{Code: "usage", Message: e.Error()}
		}
		return nil
	}
}
func (s *session) profileCommands() *cobra.Command {
	p := &cobra.Command{Use: "profile", Short: "Manage saved connections"}
	p.AddCommand(&cobra.Command{Use: "list", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, args []string) error {
		st, e := config.Load()
		if e != nil {
			return e
		}
		return s.w.Success(st, "Saved profiles", nil)
	}}, &cobra.Command{Use: "use P", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if e := config.Use(args[0]); e != nil {
			return e
		}
		return s.w.Success(map[string]string{"default": args[0]}, "Default profile updated", nil)
	}})
	return p
}

// Keep token values out of diagnostic messages, even when stdin fails.
func cleanInputError() error {
	return &output.Error{Code: "auth", Message: "could not read token from stdin", Hint: "pipe a token or run auth login in a terminal"}
}

// Reject an environment host override before reading a saved server's token.
func (s *session) checkEnvTokenURL(c config.Connection) error {
	if os.Getenv("SLOWBOOKS_URL") == "" || os.Getenv("SLOWBOOKS_TOKEN_FILE") != "" || s.flags.URLSet {
		return nil
	}
	store, err := config.Load()
	if err != nil {
		return err
	}
	p, ok := store.Profiles[c.Profile]
	if ok && strings.TrimRight(p.URL, "/") != strings.TrimRight(c.URL, "/") {
		return &output.Error{Code: "auth", Message: "environment URL differs from the selected profile", Hint: "set SLOWBOOKS_TOKEN_FILE for this server"}
	}
	return nil
}
