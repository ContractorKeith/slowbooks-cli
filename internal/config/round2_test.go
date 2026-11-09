package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestRound2RecoveryReplacesManagedToken(t *testing.T) {
	isolate(t)
	if e := SaveLogin("default", Profile{URL: "https://example.test"}, "sbp_old"); e != nil {
		t.Fatal(e)
	}
	path, _ := Path()
	if e := os.WriteFile(path, []byte(`{"default":`), 0600); e != nil {
		t.Fatal(e)
	}
	name, p, e := LoginProfile(Flags{URL: "https://example.test", URLSet: true})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(p.RecoveryNotice, "profiles.json.corrupt-") {
		t.Fatal("missing recovery notice")
	}
	if e = SaveLogin(name, p, "sbp_new"); e != nil {
		t.Fatal(e)
	}
	s, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if token, e := TokenFile(s.Profiles[name].TokenFile); e != nil || token != "sbp_new" {
		t.Fatal("managed token was not replaced", e)
	}
	backups, _ := filepath.Glob(path + ".corrupt-*")
	if len(backups) != 1 {
		t.Fatal(backups)
	}
}

func TestRound2ReloginRefusesAliasedToken(t *testing.T) {
	isolate(t)
	d, _ := Dir()
	if e := os.MkdirAll(filepath.Join(d, "tokens"), 0700); e != nil {
		t.Fatal(e)
	}
	first := filepath.Join(d, "tokens", "Acme.token")
	second := filepath.Join(d, "tokens", "acme.token")
	if e := os.WriteFile(first, []byte("sbp_original"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Link(first, second); e != nil {
		a, ae := os.Stat(first)
		b, be := os.Stat(second)
		if ae != nil || be != nil || !os.SameFile(a, b) {
			t.Fatal(e)
		}
	}
	if e := Save(Store{Default: "Acme", Profiles: map[string]Profile{"Acme": {TokenFile: first}, "acme": {TokenFile: second}}}); e != nil {
		t.Fatal(e)
	}
	_, _, e := LoginProfile(Flags{Profile: "Acme", ProfileSet: true})
	if output.ExitCode(e) != 2 || !strings.Contains(output.AsError(e).Hint, "logout") {
		t.Fatal("login resolution accepted token alias", e)
	}
	e = SaveLogin("Acme", Profile{}, "sbp_replacement")
	if output.ExitCode(e) != 2 || !strings.Contains(output.AsError(e).Hint, "logout") {
		t.Fatal("token commit accepted alias", e)
	}
	if b, e := os.ReadFile(second); e != nil || string(b) != "sbp_original" {
		t.Fatal("changed other profile credential", e)
	}
}
