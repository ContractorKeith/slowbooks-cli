package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func uniqueName(s Store, name string) error {
	if _, exists := s.Profiles[name]; exists {
		return nil
	}
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

// A managed record must name its profile's token inside the tokens directory.
// Never derive a different path from its spelling.
func managedToken(dir, name, path string) bool {
	path = tokenPath(dir, path)
	return filepath.Dir(path) == filepath.Join(dir, "tokens") && strings.EqualFold(filepath.Base(path), name+".token")
}

// Name another profile relying on this file, including aliases on
// case-insensitive filesystems and symlinks to the same file.
func sharedToken(s Store, name, dir, path string) (string, bool) {
	info, _ := os.Stat(path)
	for other, p := range s.Profiles {
		if other == name || p.TokenFile == "" {
			continue
		}
		recorded := tokenPath(dir, p.TokenFile)
		if recorded == path {
			return other, true
		}
		if info != nil {
			if otherInfo, e := os.Stat(recorded); e == nil && os.SameFile(info, otherInfo) {
				return other, true
			}
		}
	}
	return "", false
}

func refuseSharedLogin(s Store, name, dir, path string) error {
	other, shared := sharedToken(s, name, dir, path)
	if !shared {
		return nil
	}
	return &output.Error{Code: "usage", Message: "token file is shared with another profile: " + path,
		Hint: "log out the colliding profile (sbp auth logout --profile " + other + "), then re-login the remaining profile"}
}
