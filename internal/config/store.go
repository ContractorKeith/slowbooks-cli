// Package config owns connection state, paths and token files.
package config

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

type Profile struct {
	URL       string `json:"url"`
	Company   string `json:"company"`
	TokenFile string `json:"token_file"`
	Role      string `json:"role,omitempty"`
}
type Store struct {
	Default  string             `json:"default"`
	Profiles map[string]Profile `json:"profiles"`
}

// Dir returns the platform config directory, honoring the explicit override.
func Dir() (string, error) {
	if p := os.Getenv("SLOWBOOKS_CONFIG_DIR"); p != "" {
		return p, nil
	}
	p, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "sbp"), nil
}
func Path() (string, error) { p, e := Dir(); return filepath.Join(p, "profiles.json"), e }

// Load reads only profiles.json; missing state is an empty store.
func Load() (Store, error) {
	s := Store{Profiles: map[string]Profile{}}
	p, e := Path()
	if e != nil {
		return s, e
	}
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return s, &output.Error{Code: "auth", Message: "invalid profiles file", Hint: "repair profiles.json or run sbp auth login"}
	}
	if s.Profiles == nil {
		s.Profiles = map[string]Profile{}
	}
	return s, nil
}

// Save atomically replaces the profile file with private permissions.
func Save(s Store) error {
	p, e := Path()
	if e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(p, append(b, '\n'))
}

// Replace through a private file in the same directory.
func atomicWrite(path string, b []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".sbp-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

var profileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func ValidateName(name string) error {
	if !profileName.MatchString(name) {
		return &output.Error{Code: "usage", Message: "profile must contain only letters, digits, hyphens or underscores"}
	}
	return nil
}

// SaveLogin persists a validated token and records its profile.
func SaveLogin(name string, p Profile, token string) error {
	if e := ValidateName(name); e != nil {
		return e
	}
	if _, e := ValidateToken([]byte(token)); e != nil {
		return e
	}
	s, e := Load()
	if e != nil {
		return e
	}
	d, e := Dir()
	if e != nil {
		return e
	}
	p.TokenFile = filepath.Join(d, "tokens", name+".token")
	if e = atomicWrite(p.TokenFile, []byte(token+"\n")); e != nil {
		return e
	}
	s.Profiles[name] = p
	if s.Default == "" {
		s.Default = name
	}
	return Save(s)
}

// Logout removes a profile and only its managed token.
func Logout(name string) error {
	if e := ValidateName(name); e != nil {
		return e
	}
	s, e := Load()
	if e != nil {
		return e
	}
	p, ok := s.Profiles[name]
	if !ok {
		return missing()
	}
	// Only remove our managed token, never an external Switchboard token.
	d, e := Dir()
	if e != nil {
		return e
	}
	managed := filepath.Join(d, "tokens", name+".token")
	if p.TokenFile == managed {
		if e = os.Remove(managed); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	delete(s.Profiles, name)
	if s.Default == name {
		s.Default = ""
	}
	return Save(s)
}
func Use(name string) error {
	s, e := Load()
	if e != nil {
		return e
	}
	if _, ok := s.Profiles[name]; !ok {
		return missing()
	}
	s.Default = name
	return Save(s)
}
func missing() error {
	return &output.Error{Code: "auth", Message: "no configured profile", Hint: "run sbp auth login --url <server>"}
}
func CheckMode(mode os.FileMode, goos string) bool { return goos == "windows" || mode.Perm() == 0600 }

// TokenFile validates the opened file and bounds the credential read.
func TokenFile(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", &output.Error{Code: "auth", Message: "cannot read token file", Hint: "run sbp auth login"}
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() || !CheckMode(info.Mode(), runtime.GOOS) {
		return "", &output.Error{Code: "auth", Message: "unsafe token file permissions", Hint: "chmod 600 " + path}
	}
	if info.Size() > 4096 {
		return "", invalidToken()
	}
	b, e := io.ReadAll(io.LimitReader(f, 4097))
	if e != nil || len(b) > 4096 {
		return "", invalidToken()
	}
	return ValidateToken(b)
}
