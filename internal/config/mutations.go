package config

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	before := Store{Default: s.Default, Profiles: maps.Clone(s.Profiles)}
	d, e := Dir()
	if e != nil {
		return e
	}
	p.TokenFile = filepath.Join(d, "tokens", name+".token")
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
	if e = os.Rename(staged, p.TokenFile); e != nil {
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
	managed := filepath.Join(d, "tokens", name+".token")
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
	if p.TokenFile == managed {
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
