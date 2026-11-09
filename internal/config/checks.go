package config

import (
	"os"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func envOnly(f Flags) bool {
	return !f.ProfileSet && os.Getenv("SLOWBOOKS_PROFILE") == "" && os.Getenv("SLOWBOOKS_URL") != "" && os.Getenv("SLOWBOOKS_TOKEN_FILE") != ""
}

// ConfigCheck treats the profile file as optional for environment-only access.
func ConfigCheck(f Flags) (string, error) {
	if envOnly(f) {
		return "environment-only connection", nil
	}
	path, e := Path()
	if e != nil {
		return "config directory unavailable", e
	}
	s, e := Load()
	if e != nil {
		return path, e
	}
	if _, ok := s.Profiles[Selected(f, s)]; !ok {
		return path, missing()
	}
	return path, nil
}
func TokenCheck(path string) (string, error) {
	if _, e := TokenFile(path); e != nil {
		return tokenCheckFailure(e)
	}
	info, e := os.Stat(path)
	if e != nil {
		return path, fileError("inspect token file", path, e)
	}
	detail, e := tokenPermissions(path, info.Mode())
	if e != nil {
		return tokenCheckFailure(e)
	}
	return detail, nil
}

func tokenCheckFailure(e error) (string, error) {
	detail := e.Error()
	if hint := output.AsError(e).Hint; hint != "" {
		detail += "; " + hint
	}
	return detail, e
}
