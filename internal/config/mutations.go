package config

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

// SaveLogin serializes the token and profile commit, preserving prior credentials
// if staging or profile persistence fails.
func SaveLogin(name string, p Profile, token string) error {
	if e := ValidateName(name); e != nil {
		return e
	}
	token, e := ValidateToken([]byte(token))
	if e != nil {
		return e
	}
	release, e := lockStore()
	if e != nil {
		return e
	}
	defer release()
	s, e := Load()
	if e != nil {
		return e
	}
	if e = uniqueName(s, name); e != nil {
		return e
	}
	before := Store{Default: s.Default, Profiles: maps.Clone(s.Profiles)}
	d, e := Dir()
	if e != nil {
		return e
	}
	p.TokenFile = filepath.Join(d, "tokens", strings.ToLower(name)+".token")
	old, selected := s.Profiles[name]
	// Keep legacy mixed-case managed paths working without migrating credentials.
	if selected && managedToken(d, name, old.TokenFile) {
		p.TokenFile = tokenPath(d, old.TokenFile)
	}
	if _, err := os.Lstat(p.TokenFile); !os.IsNotExist(err) {
		if err != nil {
			return fileError("inspect token", p.TokenFile, err)
		}
		if !selected || !managedToken(d, name, old.TokenFile) || tokenPath(d, old.TokenFile) != p.TokenFile {
			return &output.Error{Code: "usage", Message: "token file already exists: " + p.TokenFile, Hint: "move the existing file before login: " + p.TokenFile}
		}
	}
	staged, e := stageFile(p.TokenFile, []byte(token+"\n"))
	if e != nil {
		return fileError("stage token", p.TokenFile, e)
	}
	defer os.Remove(staged)
	s.Profiles[name] = p
	if s.Default == "" {
		s.Default = name
	}
	if e = save(s); e != nil {
		return e
	}
	if e = renameFile(staged, p.TokenFile); e != nil {
		if rollbackErr := save(before); rollbackErr != nil {
			return rollbackErr
		}
		return fileError("commit token", p.TokenFile, e)
	}
	return nil
}

// Logout persists the profile removal before deleting only its managed token.
func Logout(name string) error {
	if e := ValidateName(name); e != nil {
		return e
	}
	release, e := lockStore()
	if e != nil {
		return e
	}
	defer release()
	s, e := Load()
	if e != nil {
		return e
	}
	p, ok := s.Profiles[name]
	if !ok {
		return missing()
	}
	before := Store{Default: s.Default, Profiles: maps.Clone(s.Profiles)}
	d, e := Dir()
	if e != nil {
		return e
	}
	managed := tokenPath(d, p.TokenFile)
	delete(s.Profiles, name)
	if s.Default == name {
		s.Default = ""
		names := slices.Sorted(maps.Keys(s.Profiles))
		if len(names) > 0 {
			s.Default = names[0]
		}
	}
	if e = save(s); e != nil {
		return e
	}
	if managedToken(d, name, p.TokenFile) {
		if e = os.Remove(managed); e != nil && !os.IsNotExist(e) {
			if rollbackErr := save(before); rollbackErr != nil {
				return rollbackErr
			}
			return fileError("remove token", managed, e)
		}
	}
	return nil
}
func Use(name string) error {
	release, e := lockStore()
	if e != nil {
		return e
	}
	defer release()
	s, e := Load()
	if e != nil {
		return e
	}
	if _, ok := s.Profiles[name]; !ok {
		return missing()
	}
	s.Default = name
	return save(s)
}
