//go:build !windows

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestFailedPersistencePreservesCredentials(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires ordinary filesystem permissions")
	}
	for _, location := range []string{"config", "tokens"} {
		for _, operation := range []string{"login", "logout"} {
			t.Run(operation+"/"+location, func(t *testing.T) {
				isolate(t)
				if e := SaveLogin("a", Profile{URL: "https://old", Role: "readonly"}, "sbp_old"); e != nil {
					t.Fatal(e)
				}
				original, _ := Load()
				dir, _ := Dir()
				if location == "tokens" {
					dir = filepath.Join(dir, "tokens")
				}
				if e := os.Chmod(dir, 0500); e != nil {
					t.Fatal(e)
				}
				defer os.Chmod(dir, 0700)
				var e error
				if operation == "login" {
					e = SaveLogin("a", Profile{URL: "https://new", Role: "bookkeeper"}, "sbp_new")
				} else {
					e = Logout("a")
				}
				if e == nil {
					t.Fatal("unwritable config unexpectedly succeeded")
				}
				token, readErr := TokenFile(original.Profiles["a"].TokenFile)
				if readErr != nil || token != "sbp_old" {
					t.Fatal("failed persistence changed credentials", readErr)
				}
				saved, readErr := Load()
				if readErr != nil || saved.Default != "a" || saved.Profiles["a"].URL != "https://old" {
					t.Fatal(saved, readErr)
				}
				var typed *output.Error
				var pathErr *os.PathError
				if !errors.As(e, &typed) || errors.As(e, &pathErr) || output.ExitCode(e) != 3 {
					t.Fatalf("unwrapped persistence error: %T %v", e, e)
				}
			})
		}
	}
}
