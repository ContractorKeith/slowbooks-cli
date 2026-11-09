package output

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func writeOwnedDownloadFixture(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if owner.Equals(user.User.Sid) {
		return nil
	}
	// Elevated runners may default to Administrators ownership. This fixture must
	// be owned by the user SID to exercise the cleanup's explicit ownership rule.
	if err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil); err != nil {
		return fmt.Errorf("set fixture owner from %s to %s: %w", owner.String(), user.User.Sid.String(), err)
	}
	return nil
}

func TestWindowsCleanupPreservesOtherOwner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".sbp-download-other-owner")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	owner, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	if err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, owner, nil, nil, nil); err != nil {
		t.Skipf("setting foreign owner unavailable: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err = os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	f, err := NewDownloadFile(filepath.Join(dir, "result"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if b, err := os.ReadFile(path); err != nil || string(b) != "keep" {
		t.Fatal("removed another owner's file", string(b), err)
	}
}
