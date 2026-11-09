package config

import (
	"fmt"
	"os"
	"runtime"

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
		return path + ": " + e.Error() + "; " + output.AsError(e).Hint, e
	}
	info, e := os.Stat(path)
	if e != nil {
		return path, e
	}
	detail := fmt.Sprintf("%s (%04o)", path, info.Mode().Perm())
	if runtime.GOOS == "windows" {
		detail += "; mode check skipped"
	}
	return detail, nil
}
