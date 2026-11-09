package config

import "os"

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
	name, e := ProfileName(f)
	if e != nil {
		return "", Profile{}, e
	}
	s, e := Load()
	if e != nil {
		return "", Profile{}, e
	}
	p := s.Profiles[name]
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
