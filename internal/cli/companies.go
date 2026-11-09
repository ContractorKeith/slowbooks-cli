package cli

import (
	"fmt"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
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
		for _, key := range []string{"file", "database_name", "name", "id"} {
			value, ok := row[key].(string)
			if ok && value != "" {
				if value == selected {
					return nil
				}
				names = append(names, value)
			}
		}
	}
	return &output.Error{Code: "usage", Message: fmt.Sprintf("unknown company %q", selected), Hint: "available companies: " + strings.Join(names, ", ")}
}
