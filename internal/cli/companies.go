package cli

import (
	"fmt"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/config"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

// checkCompany caches only within this invocation; company routing must not use
// a stale on-disk list. Empty selection deliberately uses the server default.
func (s *session) checkCompany(cmd *cobra.Command, cl *client.Client, always bool) error {
	selected := cl.Company()
	if selected == "" && !always {
		return nil
	}
	if !s.companiesLoaded {
		s.companiesLoaded = true
		s.companiesErr = getJSON(cmd, cl.WithoutCompany(), "/api/companies", &s.companies)
	}
	if s.companiesErr != nil {
		if always {
			return s.companiesErr
		}
		return s.warn(fmt.Sprintf("could not verify company %s: %s", selected, s.companiesErr.Error()))
	}
	if selected == "" {
		return nil
	}
	var names []string
	for _, row := range s.companies {
		file, ok := row["file"].(string)
		if !ok || file == "" {
			continue
		}
		if file == selected {
			return nil
		}
		names = append(names, file)
	}
	if len(names) == 0 {
		return &output.Error{Code: "usage", Message: "this server does not support company selection", Hint: s.companyRecoveryHint()}
	}
	return &output.Error{Code: "usage", Message: fmt.Sprintf("unknown company %q", selected), Hint: "available companies: " + strings.Join(names, ", ") + "; " + s.companyRecoveryHint()}
}

// Explain both a one-command override and permanent recovery from inherited
// environment/profile selections. Login already supports clearing with --company=.
func (s *session) companyRecoveryHint() string {
	hint := "use --company= for this invocation; unset SLOWBOOKS_COMPANY to clear an environment selection"
	if conn, err := config.Resolve(s.flags); err == nil && conn.Profile != "" {
		hint += "; clear the saved profile company with sbp auth login --profile " + conn.Profile + " --company="
	}
	return hint
}
