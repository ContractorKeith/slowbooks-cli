package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"golang.org/x/sys/windows"
)

func TestReviewDoctorNoPersistentACLs(t *testing.T) {
	share := os.Getenv("SBP_FIXB_SHARE")
	if share == "" {
		t.Skip("requires filesystem without persistent ACLs")
	}
	isolate(t)
	dir, e := os.MkdirTemp(share, "doctor-volume-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "token")
	if e = os.WriteFile(path, []byte("sbp_test"), 0600); e != nil {
		t.Fatal(e)
	}
	detail, e := TokenCheck(path)
	t.Log(detail)
	if e != nil || !strings.Contains(detail, "ACLs unsupported on this filesystem") || strings.Contains(detail, "icacls") {
		t.Fatal(detail, e)
	}
}

func TestReviewDoctorUnsafeControlGrants(t *testing.T) {
	for _, right := range []string{"WD", "WO"} {
		t.Run(right, func(t *testing.T) {
			isolate(t)
			path := filepath.Join(t.TempDir(), "token")
			if e := os.WriteFile(path, []byte("sbp_test"), 0600); e != nil {
				t.Fatal(e)
			}
			setTokenDACL(t, path, "(A;;"+right+";;;BU)")
			detail, e := TokenCheck(path)
			if output.ExitCode(e) != 3 || !strings.Contains(output.AsError(e).Hint, "icacls") {
				t.Fatal(detail, e)
			}
		})
	}
}

func TestReviewDoctorCreatorOwner(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(path, []byte("sbp_test"), 0600); e != nil {
		t.Fatal(e)
	}
	setTokenDACL(t, path, "(A;;FA;;;CO)")
	if detail, e := TokenCheck(path); e != nil {
		t.Fatal(detail, e)
	}
}

func TestReviewConfigInUseClass(t *testing.T) {
	for _, which := range []string{"profiles", "token"} {
		t.Run(which, func(t *testing.T) {
			isolate(t)
			if e := SaveLogin("a", Profile{}, "sbp_old"); e != nil {
				t.Fatal(e)
			}
			path, _ := Path()
			s, _ := Load()
			if which == "token" {
				path = s.Profiles["a"].TokenFile
			}
			ptr, _ := windows.UTF16PtrFromString(path)
			h, e := windows.CreateFile(ptr, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, 0, 0)
			if e != nil {
				t.Fatal(e)
			}
			defer windows.CloseHandle(h)
			if which == "token" {
				_, e = TokenFile(path)
			} else {
				_, e = Load()
			}
			if output.ExitCode(e) != 3 || output.AsError(e).Code != "auth" || !strings.Contains(e.Error(), "in use by another program: "+path) {
				t.Fatal(e)
			}
		})
	}
}

// Windows may substitute the real owner when setting a file ACE for CO; test
// the raw principal too, as it may be returned unchanged by other filesystems.
func TestReviewDoctorCreatorOwnerSID(t *testing.T) {
	owner, e := windows.StringToSid("S-1-5-21-1-2-3-1000")
	if e != nil {
		t.Fatal(e)
	}
	for _, value := range []string{"S-1-3-0", "S-1-3-4"} {
		sid, e := windows.StringToSid(value)
		if e != nil {
			t.Fatal(e)
		}
		if !trustedTokenPrincipal(sid, owner) {
			t.Fatal("owner principal refused", value)
		}
	}
}

func TestReviewReplaceFallbackInjected(t *testing.T) {
	for _, cause := range []error{windows.ERROR_INVALID_PARAMETER, windows.ERROR_NOT_SUPPORTED, windows.ERROR_ACCESS_DENIED} {
		t.Run(cause.Error(), func(t *testing.T) {
			dir := t.TempDir()
			old, target := filepath.Join(dir, "staged"), filepath.Join(dir, "target")
			if e := os.WriteFile(old, []byte("new"), 0600); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(target, []byte("old"), 0600); e != nil {
				t.Fatal(e)
			}
			called := false
			e := replaceFileUsing(old, target, func(source, destination string) error {
				called = true
				if source != old || destination != target {
					t.Fatal(source, destination)
				}
				return &os.LinkError{Op: "rename", Old: source, New: destination, Err: cause}
			})
			b, readErr := os.ReadFile(target)
			if !called || readErr != nil {
				t.Fatal(called, readErr)
			}
			if errors.Is(cause, windows.ERROR_ACCESS_DENIED) {
				if !errors.Is(e, cause) || string(b) != "old" {
					t.Fatal(e, string(b))
				}
				if _, e = os.Stat(old); e != nil {
					t.Fatal("source lost", e)
				}
			} else {
				if e != nil || string(b) != "new" {
					t.Fatal(e, string(b))
				}
				if _, e = os.Stat(old); !os.IsNotExist(e) {
					t.Fatal("source not renamed", e)
				}
			}
		})
	}
}
