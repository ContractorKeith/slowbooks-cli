package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func isolate(t *testing.T) {
	t.Helper()
	for _, k := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(k, t.TempDir())
	}
	for _, k := range []string{"SLOWBOOKS_URL", "SLOWBOOKS_TOKEN_FILE", "SLOWBOOKS_PROFILE", "SLOWBOOKS_COMPANY", "SLOWBOOKS_TIMEOUT_SECONDS"} {
		t.Setenv(k, "")
	}
}
func TestTokenRules(t *testing.T) {
	isolate(t)
	for _, tc := range []struct {
		value string
		valid bool
	}{{"sbp_abc", true}, {"sbp_abc\n", true}, {"sbp_abc\r\n", true}, {"sbp_", false}, {"bad", false}, {" sbp_a", false}, {"sbp_a b", false}, {"sbp_a\n\n", false}, {"sbp_a\tb", false}, {"sbp_a\r", false}, {"sbp_a\u00a0", false}} {
		_, e := ValidateToken([]byte(tc.value))
		if (e == nil) != tc.valid {
			t.Fatalf("%q: %v", tc.value, e)
		}
	}
	path := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(path, []byte("sbp_a\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := TokenFile(path); e != nil {
		t.Fatal(e)
	}
	if e := os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	_, e := TokenFile(path)
	if runtime.GOOS != "windows" && (e == nil || output.ExitCode(e) != 3 || !strings.Contains(output.AsError(e).Hint, "chmod 600 "+path)) {
		t.Fatal(e)
	}
	if !CheckMode(0644, "windows") || CheckMode(0644, "linux") || CheckMode(0644, "darwin") {
		t.Fatal("mode policy")
	}
	if _, e = TokenFile(path + "missing"); output.ExitCode(e) != 3 {
		t.Fatal(e)
	}
}
func TestStoreLifecycle(t *testing.T) {
	isolate(t)
	for _, name := range []string{"first", "second"} {
		if e := SaveLogin(name, Profile{URL: "http://localhost", Company: name, Role: "readonly"}, "sbp_test"); e != nil {
			t.Fatal(e)
		}
	}
	s, e := Load()
	if e != nil || s.Default != "first" || s.Profiles["first"].Role != "readonly" {
		t.Fatal(s, e)
	}
	path, _ := Path()
	info, _ := os.Stat(path)
	if !CheckMode(info.Mode(), runtime.GOOS) {
		t.Fatal(info.Mode())
	}
	if e = Use("second"); e != nil {
		t.Fatal(e)
	}
	s, _ = Load()
	if s.Default != "second" {
		t.Fatal(s)
	}
	token := s.Profiles["second"].TokenFile
	if e = Logout("second"); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(token); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	if e = SaveLogin("../escape", Profile{}, "sbp_test"); output.ExitCode(e) != 2 {
		t.Fatal(e)
	}
	if e = Use("missing"); output.ExitCode(e) != 3 {
		t.Fatal(e)
	}
}
func TestPrecedenceAndEnvOnly(t *testing.T) {
	isolate(t)
	if _, e := Resolve(Flags{}); output.ExitCode(e) != 3 {
		t.Fatal(e)
	}
	if e := SaveLogin("a", Profile{URL: "https://profile", Company: "profile"}, "sbp_a"); e != nil {
		t.Fatal(e)
	}
	c, e := Resolve(Flags{})
	if e != nil || c.URL != "https://profile" || c.Company != "profile" || c.Timeout != 120*time.Second {
		t.Fatal(c, e)
	}
	t.Setenv("SLOWBOOKS_URL", "https://env")
	t.Setenv("SLOWBOOKS_COMPANY", "env")
	t.Setenv("SLOWBOOKS_TOKEN_FILE", "env.token")
	t.Setenv("SLOWBOOKS_TIMEOUT_SECONDS", "17")
	c, e = Resolve(Flags{})
	if e != nil || c.URL != "https://env" || c.Company != "env" || c.TokenFile != "env.token" || c.Timeout != 17*time.Second {
		t.Fatal(c, e)
	}
	c, e = Resolve(Flags{Profile: "a", ProfileSet: true, URL: "https://flag", URLSet: true, Company: "", CompanySet: true, Timeout: 5, TimeoutSet: true})
	if e != nil || c.URL != "https://flag" || c.Company != "" || c.Timeout != 5*time.Second {
		t.Fatal(c, e)
	}
	path, _ := Path()
	if e = os.WriteFile(path, []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Resolve(Flags{}); e != nil {
		t.Fatal("env-only read profiles", e)
	}
	t.Setenv("SLOWBOOKS_PROFILE", "a")
	if _, e = Resolve(Flags{}); e == nil {
		t.Fatal("ignored selected profile")
	}
}
func TestTimeoutAndPaths(t *testing.T) {
	isolate(t)
	for _, v := range []string{"0", "3601", "bad", "-1"} {
		t.Setenv("SLOWBOOKS_TIMEOUT_SECONDS", v)
		if _, e := Timeout(Flags{}); output.ExitCode(e) != 2 {
			t.Fatal(v, e)
		}
		if _, e := Timeout(Flags{Timeout: 1, TimeoutSet: true}); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{"1", "3600"} {
		t.Setenv("SLOWBOOKS_TIMEOUT_SECONDS", v)
		if _, e := Timeout(Flags{}); e != nil {
			t.Fatal(e)
		}
	}
	t.Setenv("SLOWBOOKS_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	base, e := os.UserConfigDir()
	if e != nil {
		t.Fatal(e)
	}
	p, e := Dir()
	if e != nil || p != filepath.Join(base, "slowbooks") {
		t.Fatal(p, e)
	}
}

func TestProfileSelectionAndExternalTokenPreservation(t *testing.T) {
	isolate(t)
	for _, name := range []string{"a", "b"} {
		if e := SaveLogin(name, Profile{URL: "https://" + name}, "sbp_test"); e != nil {
			t.Fatal(e)
		}
	}
	t.Setenv("SLOWBOOKS_PROFILE", "b")
	c, e := Resolve(Flags{})
	if e != nil || c.Profile != "b" || c.URL != "https://b" {
		t.Fatal(c, e)
	}
	c, e = Resolve(Flags{Profile: "a", ProfileSet: true})
	if e != nil || c.Profile != "a" || c.URL != "https://a" {
		t.Fatal(c, e)
	}
	s, _ := Load()
	external := filepath.Join(t.TempDir(), "switchboard.token")
	if e = os.WriteFile(external, []byte("sbp_external"), 0600); e != nil {
		t.Fatal(e)
	}
	p := s.Profiles["a"]
	p.TokenFile = external
	s.Profiles["a"] = p
	if e = Save(s); e != nil {
		t.Fatal(e)
	}
	if e = Logout("a"); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(external); e != nil {
		t.Fatal("removed external token", e)
	}
}
func TestSaveReplacesPrivately(t *testing.T) {
	isolate(t)
	if e := SaveLogin("a", Profile{URL: "https://a"}, "sbp_first"); e != nil {
		t.Fatal(e)
	}
	s, _ := Load()
	path := s.Profiles["a"].TokenFile
	if e := os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	if e := SaveLogin("a", Profile{URL: "https://b"}, "sbp_second"); e != nil {
		t.Fatal(e)
	}
	token, e := TokenFile(path)
	if e != nil || token != "sbp_second" {
		t.Fatal(token, e)
	}
	s, _ = Load()
	if s.Profiles["a"].URL != "https://b" {
		t.Fatal(s)
	}
}
