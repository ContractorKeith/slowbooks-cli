// Package config owns connection state, paths and token files.
package config

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

type Profile struct {
	recovering     bool   // One login attempt may reclaim its managed token; never persisted.
	RecoveryNotice string `json:"-"`
	URL            string `json:"url"`
	Company        string `json:"company"`
	TokenFile      string `json:"token_file"`
	Role           string `json:"role,omitempty"`
}
type Store struct {
	Default  string             `json:"default"`
	Profiles map[string]Profile `json:"profiles"`
}

// Dir returns the platform config directory, honoring the explicit override.
func Dir() (string, error) {
	if p := os.Getenv("SLOWBOOKS_CONFIG_DIR"); p != "" {
		return filepath.Abs(p)
	}
	p, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(p, "slowbooks"), nil
}
func Path() (string, error) { p, e := Dir(); return filepath.Join(p, "profiles.json"), e }

// Load reads only profiles.json; missing state is an empty store.
func Load() (Store, error) {
	s := Store{Profiles: map[string]Profile{}}
	p, e := Path()
	if e != nil {
		return s, e
	}
	info, e := os.Lstat(p)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, fileError("inspect profiles file", p, e)
	}
	if !info.Mode().IsRegular() {
		return s, nonRegularProfiles(p)
	}
	f, e := openProfiles(p)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, fileError("read profiles file", p, e)
	}
	defer f.Close()
	info, e = f.Stat()
	if e != nil {
		return s, fileError("inspect profiles file", p, e)
	}
	if !info.Mode().IsRegular() {
		return s, nonRegularProfiles(p)
	}
	b, e := io.ReadAll(f)
	if e != nil {
		return s, fileError("read profiles file", p, e)
	}
	if e = json.Unmarshal(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}), &s); e != nil {
		return s, &output.Error{Code: "auth", Message: "invalid profiles file: " + p, Hint: "run slowbooks auth login --url <server> to recover " + p}
	}
	if s.Profiles == nil {
		s.Profiles = map[string]Profile{}
	}
	return s, nil
}

// Save atomically replaces the profile file with private permissions.
func Save(s Store) error {
	release, e := lockStore()
	if e != nil {
		return e
	}
	defer release()
	return save(s)
}
func save(s Store) error {
	p, e := Path()
	if e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	if e = atomicWrite(p, append(b, '\n')); e != nil {
		return fileError("save profiles", p, e)
	}
	return nil
}

var profileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func ValidateName(name string) error {
	if !profileName.MatchString(name) {
		return &output.Error{Code: "usage", Message: "profile must contain only letters, digits, hyphens or underscores"}
	}
	return nil
}

func missing() error {
	return &output.Error{Code: "auth", Message: "no configured profile", Hint: "run slowbooks auth login --url <server>"}
}

func nonRegularProfiles(path string) error {
	return &output.Error{Code: "auth", Message: "profiles file is not a regular file: " + path, Hint: "use a regular profiles.json file: " + path}
}
