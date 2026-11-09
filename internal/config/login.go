package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ProfileName selects a saved profile, or the initial login name.
func ProfileName(f Flags) (string, error) {
	s, e := Load()
	if e != nil {
		return "", e
	}
	name := Selected(f, s)
	if name == "" {
		name = "default"
	}
	return name, ValidateName(name)
}

// LoginProfile resolves flags and environment before a token has been saved.
func LoginProfile(f Flags) (string, Profile, error) {
	s, notice, e := loginStore()
	if e != nil {
		return "", Profile{}, e
	}
	name := Selected(f, s)
	if name == "" {
		name = "default"
	}
	if e = ValidateName(name); e != nil {
		return "", Profile{}, e
	}
	if e = uniqueName(s, name); e != nil {
		return "", Profile{}, e
	}
	p := s.Profiles[name]
	p.RecoveryNotice = notice
	p.recovering = notice != ""
	if p.recovering {
		dir, err := Dir()
		if err != nil {
			return "", Profile{}, err
		}
		token := filepath.Join(dir, "tokens", strings.ToLower(name)+".token")
		if _, err = os.Lstat(token); err == nil {
			p.RecoveryNotice += "; recovery will replace managed token: " + token
		}
	}
	if p.TokenFile != "" {
		dir, err := Dir()
		if err != nil {
			return "", Profile{}, err
		}
		if err = refuseSharedLogin(s, name, dir, tokenPath(dir, p.TokenFile)); err != nil {
			return "", Profile{}, err
		}
	}
	p.URL = first(os.Getenv("SLOWBOOKS_URL"), p.URL)
	if f.URLSet {
		p.URL = f.URL
	}
	p.Company = first(os.Getenv("SLOWBOOKS_COMPANY"), p.Company)
	if f.CompanySet {
		p.Company = f.Company
	}
	return name, p, nil
}
