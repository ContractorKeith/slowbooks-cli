package cli

import (
	"io"
	"os"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/config"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (s *session) authCommands() *cobra.Command {
	a := &cobra.Command{Use: "auth", Short: "Manage API token authentication"}
	login := &cobra.Command{Use: "login", Short: "Save a verified server connection", Args: usageArgs(cobra.NoArgs), RunE: s.login}
	login.Flags().StringVar(&s.flags.URL, "url", "", "Server URL")
	a.AddCommand(login, &cobra.Command{Use: "status", Short: "Show the current connection and role", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, args []string) error {
		conn, c, e := s.connect(cmd)
		if e != nil {
			return e
		}
		v, r, e := s.inspect(cmd, c)
		if e != nil {
			return e
		}
		return s.w.Success(status{conn.URL, conn.Company, r, v, conn.TokenFile}, "Authenticated", nil)
	}}, &cobra.Command{Use: "logout", Short: "Remove a saved profile and its token", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, args []string) error {
		name, e := config.ProfileName(s.flags)
		if e != nil {
			return e
		}
		if e = config.Logout(name); e != nil {
			return e
		}
		st, e := config.Load()
		if e != nil {
			return e
		}
		summary := "Logged out"
		if st.Default != "" {
			summary += "; default profile: " + st.Default
		}
		return s.w.Success(map[string]string{"profile": name}, summary, nil)
	}})
	return a
}
func (s *session) login(cmd *cobra.Command, args []string) error {
	name, p, e := config.LoginProfile(s.flags)
	if e != nil {
		return e
	}
	if e = s.warn(p.RecoveryNotice); e != nil {
		return e
	}
	if strings.TrimSpace(p.URL) == "" {
		return &output.Error{Code: "usage", Message: "server URL is required", Hint: "run slowbooks auth login --url <server>"}
	}
	timeout, e := config.Timeout(s.flags)
	if e != nil {
		return e
	}
	var raw []byte
	input := cmd.InOrStdin()
	if f, ok := input.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if e = output.New(s.w.Err, s.w.Err).Text("Token: "); e != nil {
			return e
		}
		raw, e = term.ReadPassword(int(f.Fd()))
		_ = output.New(s.w.Err, s.w.Err).Text("\n")
	} else {
		raw, e = io.ReadAll(io.LimitReader(input, config.MaxTokenFileSize+1))
	}
	if e != nil {
		return cleanInputError()
	}
	if len(raw) > config.MaxTokenFileSize {
		return cleanInputError()
	}
	token, e := config.ValidateToken(raw)
	if e != nil {
		return e
	}
	c, e := client.New(cmd.Context(), client.Options{URL: p.URL, Company: p.Company, Token: token, Timeout: timeout, Version: s.build.Version})
	if e != nil {
		return e
	}
	v, r, e := s.inspect(cmd, c)
	if e != nil {
		return e
	}
	if e = s.checkCompany(cmd, c, false); e != nil {
		return e
	}
	p.Role = r
	if e = config.SaveLogin(name, p, token); e != nil {
		return e
	}
	st, e := config.Load()
	if e != nil {
		return e
	}
	p = st.Profiles[name]
	return s.w.Success(status{p.URL, p.Company, r, v, p.TokenFile}, "Logged in", []string{"slowbooks doctor"})
}
