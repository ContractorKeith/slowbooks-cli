package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossBuildRunsEveryTargetAndStopsOnFailure(t *testing.T) {
	bash := scriptBash(t)
	dir := t.TempDir()
	mock := filepath.Join(dir, "go")
	log := filepath.Join(dir, "calls")
	script := `#!/usr/bin/env bash
printf '%s/%s CGO=%s %s\n' "$GOOS" "$GOARCH" "$CGO_ENABLED" "$*" >> "$BUILD_TEST_LOG"
if test "$GOOS/$GOARCH" = "$BUILD_TEST_FAIL"; then exit 17; fi
`
	if err := os.WriteFile(mock, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, fail := range []string{"", "linux/arm64"} {
		if err := os.WriteFile(log, nil, 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bash, "../../scripts/cross-build.sh")
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "BUILD_TEST_LOG="+log, "BUILD_TEST_FAIL="+fail)
		raw, err := cmd.CombinedOutput()
		calls, readErr := os.ReadFile(log)
		if readErr != nil {
			t.Fatal(readErr)
		}
		lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
		targets := []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"}
		if fail != "" {
			targets = targets[:2]
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 17 {
				t.Fatalf("%v %s", err, raw)
			}
		} else if err != nil {
			t.Fatalf("%v %s", err, raw)
		}
		if len(lines) != len(targets) {
			t.Fatal(string(calls))
		}
		for i, target := range targets {
			if lines[i] != target+" CGO=0 build ./..." {
				t.Fatal(string(calls))
			}
		}
	}
	workflow, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "run: bash scripts/cross-build.sh") {
		t.Fatal("CI does not run the complete cross-build script")
	}
}
