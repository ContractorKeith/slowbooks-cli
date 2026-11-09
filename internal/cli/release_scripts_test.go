package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func scriptBash(t *testing.T) string {
	t.Helper()
	isolate(t)
	if runtime.GOOS == "windows" {
		t.Skip("release/smoke Bash scripts run on Unix")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash unavailable")
	}
	return bash
}

func TestSmokeCleanupExitAndSignals(t *testing.T) {
	bash := scriptBash(t)
	script, err := filepath.Abs("../../scripts/smoke-live.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, before, after    string
		voidFails, createFails bool
		want                   int
	}{
		{name: "success"},
		{name: "failure after create", before: `printf() { case "$1" in 'created invoice id='*) return 23;; *) builtin printf "$@";; esac; }; `, want: 23},
		{name: "interrupt after create", before: `printf() { case "$1" in 'created invoice id='*) kill -INT $$;; *) builtin printf "$@";; esac; }; `, want: 130},
		{name: "terminate after create", before: `printf() { case "$1" in 'created invoice id='*) kill -TERM $$;; *) builtin printf "$@";; esac; }; `, want: 143},
		{name: "exit failure", after: "exit 23", want: 23},
		{name: "interrupt", after: "kill -INT $$", want: 130},
		{name: "terminate", after: "kill -TERM $$", want: 143},
		{name: "void failure", voidFails: true, want: 1},
		{name: "original failure preserved", after: "exit 23", voidFails: true, want: 23},
		{name: "create failure", createFails: true, want: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mock := filepath.Join(dir, "sbp")
			log := filepath.Join(dir, "calls")
			raw := `#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$MOCK_LOG"
case "$1 ${2:-}" in
 'customers list') echo 7;;
 'invoices create')
   case " $* " in *' --dry-run '*) exit 0;; esac
   if test "$MOCK_CREATE_FAIL" = true; then echo '{"ok":false,"error":{"code":"network"}}'; exit 8; fi
   echo 42;;
 'invoices void') if test "$MOCK_VOID_FAIL" = true; then exit 8; fi;;
esac
`
			if err := os.WriteFile(mock, []byte(raw), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bash, "-c", tc.before+`source "$SBP_SMOKE_SCRIPT"; `+tc.after+"\n")
			// A trailing no-op gives the sourced success case an explicit final command.
			if tc.after == "" {
				cmd = exec.Command(bash, "-c", tc.before+`source "$SBP_SMOKE_SCRIPT"; :`)
			}
			boolString := func(v bool) string {
				if v {
					return "true"
				}
				return "false"
			}
			cmd.Env = append(os.Environ(), "CI=", "SLOWBOOKS_URL=http://127.0.0.1", "SLOWBOOKS_TOKEN_FILE=unused", "SBP_BIN="+mock, "SBP_SMOKE_SCRIPT="+script, "MOCK_LOG="+log, "MOCK_VOID_FAIL="+boolString(tc.voidFails), "MOCK_CREATE_FAIL="+boolString(tc.createFails))
			out, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					code = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tc.want {
				t.Fatalf("exit %d want %d: %s", code, tc.want, out)
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			wantVoids := 1
			if tc.createFails {
				wantVoids = 0
			}
			if n := bytes.Count(calls, []byte("invoices void ")); n != wantVoids {
				t.Fatalf("%d voids want %d: %s", n, wantVoids, calls)
			}
			if tc.voidFails && !bytes.Contains(out, []byte("could not confirm void for invoice id=42; inspect it and, if needed, run: sbp invoices void 42 --json")) {
				t.Fatalf("missing manual recovery hint: %s", out)
			}
		})
	}
}

func TestReleaseVersionGate(t *testing.T) {
	bash := scriptBash(t)
	for _, version := range []string{"v0.0.0", "v1.2.3", "v0.1.0-rc.1", "v1.2.3+build.1", "v01.2.3", "v1.2", "bad", ""} {
		out, err := exec.Command(bash, "../../scripts/check-release-version.sh", version).CombinedOutput()
		valid := version == "v0.0.0" || version == "v1.2.3"
		if valid && err != nil {
			t.Fatalf("%s: %v %s", version, err, out)
		}
		if !valid {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 || !bytes.Contains(out, []byte("requires a stable vX.Y.Z tag")) {
				t.Fatalf("%s: %v %s", version, err, out)
			}
		}
	}
	cmd := exec.Command(bash, "../../scripts/release-dry-run.sh")
	cmd.Env = append(os.Environ(), "SBP_RELEASE_VERSION=v0.1.0-rc.1")
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 || !bytes.Contains(out, []byte("requires a stable vX.Y.Z tag")) {
		t.Fatalf("prerelease build did not fail early: %v %s", err, out)
	}
}

func TestReleaseWorkflowRequiresTestsBeforeBuild(t *testing.T) {
	isolate(t)
	raw, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	previous := -1
	for _, step := range []string{"bash scripts/check-release-version.sh", "go install honnef.co/go/tools/cmd/staticcheck@v0.8.2", "run: go test ./...", "run: bash scripts/accept.sh", "name: Build archives, checksums and manifest"} {
		index := strings.Index(text, step)
		if index <= previous {
			t.Fatalf("missing or out-of-order release gate: %s", step)
		}
		previous = index
	}
	if !strings.Contains(text, "if: needs.artifacts.outputs.stable == 'true'") {
		t.Fatal("publisher lacks stable-version gate")
	}
}

func TestReleaseScriptsExecutable(t *testing.T) {
	scriptBash(t)
	for _, path := range []string{"../../scripts/release-dry-run.sh", "../../scripts/smoke-live.sh"} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&0111 == 0 {
			t.Fatal("not executable", path)
		}
	}
}
