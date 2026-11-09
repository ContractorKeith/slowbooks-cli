package cli

import (
	"fmt"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/config"
	"github.com/spf13/cobra"
)

type check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}
type checksFailed struct{ cause error }

func (e *checksFailed) Error() string { return e.cause.Error() }
func (s *session) doctorCommand() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check connection health", Args: usageArgs(cobra.NoArgs), RunE: s.doctor}
}
func (s *session) doctor(cmd *cobra.Command, args []string) error {
	checks := []check{}
	var first error
	add := func(name, detail string, e error) {
		checks = append(checks, check{name, e == nil, detail})
		if first == nil && e != nil {
			first = e
		}
	}
	detail, configErr := config.ConfigCheck(s.flags)
	add("config file", detail, configErr)
	conn, resolveErr := config.Resolve(s.flags)
	var c *client.Client
	var e error
	if resolveErr != nil {
		add("token file", "connection unavailable", resolveErr)
	} else {
		detail, tokenErr := config.TokenCheck(conn.TokenFile)
		add("token file", detail, tokenErr)
		if tokenErr == nil {
			_, c, e = s.connect(cmd)
		}
	}
	if c == nil {
		if e != nil {
			add("reachability", e.Error(), e)
		} else {
			checks = append(checks, check{"reachability", false, "connection unavailable"})
		}
		for _, name := range []string{"version", "role", "companies"} {
			checks = append(checks, check{name, false, "connection unavailable"})
		}
	} else {
		var system struct {
			Version string `json:"version"`
		}
		systemErr := getJSON(cmd, c, "/api/system", &system)
		detail := "reachable"
		if systemErr != nil {
			detail = systemErr.Error()
		}
		add("reachability", detail, systemErr)
		if systemErr != nil {
			checks = append(checks, check{"version", false, "unavailable"})
		} else {
			warning := versionWarning(system.Version)
			detail = system.Version
			if warning != "" {
				detail += "; warning: " + warning
			}
			if e = s.warn(warning); e != nil {
				return e
			}
			add("version", detail, nil)
		}
		r, roleErr := role(cmd, c)
		if roleErr != nil {
			r = roleErr.Error()
		}
		add("role", r, roleErr)
		companyErr := s.checkCompany(cmd, c, true)
		detail = "available"
		if companyErr != nil {
			detail = companyErr.Error()
		}
		add("companies", detail, companyErr)
	}
	if c == nil {
		checks = append(checks, check{"catalog cache", false, "connection unavailable"})
	} else {
		cat, err := catalog.Load(cmd.Context(), c, conn.Profile, false)
		detail := "unavailable"
		if err == nil {
			detail = fmt.Sprintf("version %s; %d operations; %s", cat.Version, len(cat.Operations), cat.Path)
			if cat.Warning != "" {
				detail += "; " + cat.Warning
				if err := s.warn(cat.Warning); err != nil {
					return err
				}
			}
		}
		add("catalog cache", detail, err)
	}
	var data any = struct {
		Checks []check `json:"checks"`
	}{checks}
	if s.w.TTY && !s.w.JSON && !s.w.Quiet {
		rows := map[string]string{}
		for _, check := range checks {
			state := "ok"
			if !check.OK {
				state = "failed"
			}
			rows[check.Name] = state + "; " + check.Detail
		}
		data = rows
	}
	if e = s.w.Success(data, "Connection checks", nil); e != nil {
		return e
	}
	if first != nil {
		return &checksFailed{first}
	}
	return nil
}
