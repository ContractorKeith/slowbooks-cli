package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Opt in because this reruns the complete gate. Poisoning the saved URL must
// break the goldens, proving they actually consume profiles instead of env URL.
func TestAcceptanceGoldensUseSavedProfile(t *testing.T) {
	if os.Getenv("SBP_ACCEPT_PROFILE_PROBE") != "1" {
		t.Skip("set SBP_ACCEPT_PROFILE_PROBE=1 for the harness regression")
	}
	caches, e := exec.Command("go", "env", "-json", "GOCACHE", "GOMODCACHE").Output()
	if e != nil {
		t.Fatal(e)
	}
	var values map[string]string
	if e = json.Unmarshal(caches, &values); e != nil {
		t.Fatal(e)
	}
	isolate(t)
	t.Setenv("GOCACHE", values["GOCACHE"])
	t.Setenv("GOMODCACHE", values["GOMODCACHE"])
	t.Setenv("SBP_ACCEPT_PROFILE_PROBE", "")
	hook := filepath.Join(t.TempDir(), "probe.bash")
	content := `diff() {
 result=0
 command diff "$@" || result=$?
 case "$*" in
  '- testdata/golden/version.json')
   python3 -c 'import json,os,shutil; p=os.path.join(os.environ["SLOWBOOKS_CONFIG_DIR"],"profiles.json"); shutil.copyfile(p,p+".probe"); s=json.load(open(p)); s["profiles"][s["default"]]["url"]="http://127.0.0.1:1"; f=open(p,"w"); json.dump(s,f); f.close()'
   builtin echo 'profile-probe: poisoned saved URL'
   ;;
  '- testdata/golden/auth-status.json')
   python3 -c 'import os,shutil; p=os.path.join(os.environ["SLOWBOOKS_CONFIG_DIR"],"profiles.json"); shutil.copyfile(p+".probe",p)'
   ;;
 esac
 return "$result"
}
`
	if e = os.WriteFile(hook, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("BASH_ENV", hook)
	cmd := exec.Command("bash", "scripts/accept.sh")
	cmd.Dir = filepath.Join("..", "..")
	out, e := cmd.CombinedOutput()
	if e == nil {
		t.Fatal("goldens passed despite an unusable saved profile")
	}
	if !strings.Contains(string(out), "profile-probe: poisoned saved URL") || !strings.Contains(string(out), "request failed or timed out") {
		t.Fatalf("failed outside the profile probe: %v\n%s", e, out)
	}
}

func TestQAWorktreesIgnored(t *testing.T) {
	isolate(t)
	cmd := exec.Command("git", "check-ignore", "--no-index", ".wt/qa-regression")
	cmd.Dir = filepath.Join("..", "..")
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("QA worktree not ignored: %v %s", e, out)
	}
}

func TestAcceptanceNormalizerPreservesMoneySpelling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("accept.sh runs only on Linux/macOS; Windows does not run the shell acceptance harness")
	}
	isolate(t)
	raw, err := os.ReadFile("../../scripts/accept.sh")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "normalize() {")
	end := strings.Index(source[start:], "\n}") + start + 2
	cmd := exec.Command("bash", "-c", source[start:end]+"\nnormalize")
	cmd.Env = append(os.Environ(), "fake_url=http://127.0.0.1:1234")
	cmd.Stdin = strings.NewReader(`{"url":"http://127.0.0.1:1234/api/payments","amount":1234.50,"allocation":0.10,"header":"Bearer \u003credacted\u003e"}` + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("normalizer: %v %s", err, out)
	}
	expected := `{"url":"<server>/api/payments","amount":1234.50,"allocation":0.10,"header":"Bearer \u003credacted\u003e"}` + "\n"
	if string(out) != expected {
		t.Fatalf("normalizer changed money or JSON escaping:\n%s", out)
	}
}
