package config

import (
	"path/filepath"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func uniqueName(s Store, name string) error {
	for existing := range s.Profiles {
		if existing != name && strings.EqualFold(existing, name) {
			return &output.Error{Code: "usage", Message: "profile already exists with different case: " + existing, Hint: "use --profile " + existing}
		}
	}
	return nil
}
func tokenPath(dir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(dir, path)
}

// Recognize the original spelling as well as the folded path for legacy stores.
func managedToken(dir, name, path string) bool {
	path = tokenPath(dir, path)
	return path == filepath.Join(dir, "tokens", name+".token") || path == filepath.Join(dir, "tokens", strings.ToLower(name)+".token")
}
