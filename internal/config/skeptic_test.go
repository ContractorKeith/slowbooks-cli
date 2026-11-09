package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestCaseUniqueProfiles(t *testing.T) {
	isolate(t)
	if e := SaveLogin("Acme", Profile{URL: "https://example.test"}, "sbp_first"); e != nil {
		t.Fatal(e)
	}
	s, _ := Load()
	if filepath.Base(s.Profiles["Acme"].TokenFile) != "acme.token" {
		t.Error(s)
	}
	if e := SaveLogin("acme", Profile{}, "sbp_second"); output.ExitCode(e) != 2 || output.AsError(e).Hint == "" {
		t.Fatal(e)
	}
	c, e := Resolve(Flags{})
	if e != nil || c.Profile != "acme" {
		t.Fatal(c, e)
	}
}
func TestForeignTokenRefused(t *testing.T) {
	isolate(t)
	d, _ := Dir()
	p := filepath.Join(d, "tokens", "acme.token")
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte("sbp_foreign"), 0600); e != nil {
		t.Fatal(e)
	}
	e := SaveLogin("acme", Profile{}, "sbp_new")
	if output.ExitCode(e) != 2 || !strings.Contains(output.AsError(e).Hint, p) {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "sbp_foreign" {
		t.Fatal(string(b))
	}
}
func TestLoginRecovery(t *testing.T) {
	for _, raw := range []string{"", `{"profiles":`, "\xef\xbb\xbf"} {
		t.Run(raw, func(t *testing.T) {
			isolate(t)
			p, _ := Path()
			if e := os.WriteFile(p, []byte(raw), 0600); e != nil {
				t.Fatal(e)
			}
			_, e := Load()
			if e == nil || !strings.Contains(output.AsError(e).Hint, p) {
				t.Fatal(e)
			}
			if _, p, e := LoginProfile(Flags{}); e != nil || !strings.Contains(p.RecoveryNotice, "profiles.json.corrupt-") {
				t.Fatal(e)
			}
			if e := SaveLogin("default", Profile{}, "sbp_test"); e != nil {
				t.Fatal(e)
			}
			backups, _ := filepath.Glob(p + ".corrupt-*")
			if len(backups) != 1 {
				t.Fatal(backups)
			}
			b, _ := os.ReadFile(backups[0])
			if string(b) != raw {
				t.Fatal(string(b))
			}
		})
	}
}
func TestBOMAndRelativeToken(t *testing.T) {
	isolate(t)
	d, _ := Dir()
	p, _ := Path()
	raw := "\xef\xbb\xbf" + `{"default":"a","profiles":{"a":{"url":"https://example.test","token_file":"a.token"}}}`
	if e := os.WriteFile(p, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	token := filepath.Join(d, "a.token")
	if e := os.WriteFile(token, []byte("\xef\xbb\xbfsbp_test\r\n"), 0600); e != nil {
		t.Fatal(e)
	}
	c, e := Resolve(Flags{})
	if e != nil || c.TokenFile != token {
		t.Fatal(c, e)
	}
	if s, e := TokenFile(token); e != nil || s != "sbp_test" {
		t.Fatal(s, e)
	}
	_, e = ValidateToken([]byte{255, 254, 's', 0})
	if e == nil || !strings.Contains(e.Error(), "UTF-16 is not supported") {
		t.Fatal(e)
	}
}

func TestLegacyManagedTokenAndExternalLogout(t *testing.T) {
	isolate(t)
	d, _ := Dir()
	legacy := filepath.Join(d, "tokens", "Acme.token")
	if e := os.MkdirAll(filepath.Dir(legacy), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(legacy, []byte("sbp_old"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := Save(Store{Default: "Acme", Profiles: map[string]Profile{"Acme": {URL: "https://example.test", TokenFile: legacy}}}); e != nil {
		t.Fatal(e)
	}
	c, e := Resolve(Flags{})
	if e != nil || c.TokenFile != legacy {
		t.Fatal(c, e)
	}
	if e = SaveLogin("Acme", Profile{URL: c.URL}, "sbp_new"); e != nil {
		t.Fatal(e)
	}
	if token, e := TokenFile(legacy); e != nil || token != "sbp_new" {
		t.Fatal(token, e)
	}
	if e = Logout("Acme"); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(legacy); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	external := filepath.Join(d, "tokens", "external.token")
	if e = os.WriteFile(external, []byte("sbp_external"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = Save(Store{Default: "a", Profiles: map[string]Profile{"a": {TokenFile: "tokens/external.token"}}}); e != nil {
		t.Fatal(e)
	}
	if e = Logout("a"); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(external); e != nil || string(b) != "sbp_external" {
		t.Fatal(string(b), e)
	}
}
