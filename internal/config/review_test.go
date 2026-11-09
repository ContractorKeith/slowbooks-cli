package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestReloginLegacyCaseCollisions(t *testing.T) {
	isolate(t)
	d, _ := Dir()
	if e := os.MkdirAll(filepath.Join(d, "tokens"), 0700); e != nil {
		t.Fatal(e)
	}
	profiles := map[string]Profile{}
	for _, name := range []string{"Acme", "acme"} {
		path := filepath.Join(d, "tokens", name+".token")
		if e := os.WriteFile(path, []byte("sbp_old"), 0600); e != nil {
			t.Fatal(e)
		}
		profiles[name] = Profile{TokenFile: path}
	}
	if e := Save(Store{Default: "Acme", Profiles: profiles}); e != nil {
		t.Fatal(e)
	}
	a, _ := os.Stat(profiles["Acme"].TokenFile)
	b, _ := os.Stat(profiles["acme"].TokenFile)
	aliased := os.SameFile(a, b)
	for _, name := range []string{"Acme", "acme"} {
		if aliased {
			if _, _, e := LoginProfile(Flags{Profile: name, ProfileSet: true}); output.ExitCode(e) != 2 {
				t.Fatal("accepted case-insensitive alias", e)
			}
			if e := SaveLogin(name, Profile{}, "sbp_test"); output.ExitCode(e) != 2 {
				t.Fatal("overwrote case-insensitive alias", e)
			}
			continue
		}
		if _, _, e := LoginProfile(Flags{Profile: name, ProfileSet: true}); e != nil {
			t.Fatal(name, e)
		}
		if e := SaveLogin(name, Profile{}, "sbp_test"); e != nil {
			t.Fatal(name, e)
		}
	}
	if e := SaveLogin("ACME", Profile{}, "sbp_test"); output.ExitCode(e) != 2 {
		t.Fatal(e)
	}
}

func TestLogoutPreservesCollidingRecordedToken(t *testing.T) {
	isolate(t)
	d, _ := Dir()
	path := filepath.Join(d, "tokens", "acme.token")
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte("sbp_keep"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := Save(Store{Default: "Acme", Profiles: map[string]Profile{"Acme": {TokenFile: path}, "acme": {TokenFile: path}}}); e != nil {
		t.Fatal(e)
	}
	if e := Logout("Acme"); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(path); e != nil || string(b) != "sbp_keep" {
		t.Fatal("deleted colliding profile token", string(b), e)
	}
}

func TestRecoveryPreservesSymlink(t *testing.T) {
	isolate(t)
	path, _ := Path()
	target := filepath.Join(t.TempDir(), "original")
	if e := os.WriteFile(target, []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(target, path); e != nil {
		t.Skipf("symlink unavailable: %v", e)
	}
	_, _, e := LoginProfile(Flags{})
	if output.ExitCode(e) != 3 || !strings.Contains(e.Error(), path) {
		t.Fatal(e)
	}
	if info, e := os.Lstat(path); e != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("changed symlink", info, e)
	}
	backups, _ := filepath.Glob(path + ".corrupt-*")
	if len(backups) != 0 {
		t.Fatal(backups)
	}
}

func TestLogoutPreservesOtherCasePath(t *testing.T) {
	isolate(t)
	d, _ := Dir()
	if e := os.MkdirAll(filepath.Join(d, "tokens"), 0700); e != nil {
		t.Fatal(e)
	}
	profiles := map[string]Profile{}
	for _, name := range []string{"Acme", "acme"} {
		path := filepath.Join(d, "tokens", name+".token")
		if e := os.WriteFile(path, []byte("sbp_keep"), 0600); e != nil {
			t.Fatal(e)
		}
		profiles[name] = Profile{TokenFile: path}
	}
	if e := Save(Store{Default: "Acme", Profiles: profiles}); e != nil {
		t.Fatal(e)
	}
	if e := Logout("Acme"); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(profiles["acme"].TokenFile); e != nil || string(b) != "sbp_keep" {
		t.Fatal("deleted another case path", string(b), e)
	}
}
