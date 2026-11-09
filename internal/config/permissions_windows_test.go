package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"golang.org/x/sys/windows"
)

func setTokenDACL(t *testing.T, path, extra string) {
	t.Helper()
	sd, e := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if e != nil {
		t.Fatal(e)
	}
	owner, _, e := sd.Owner()
	if e != nil {
		t.Fatal(e)
	}
	descriptor, e := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + owner.String() + ")(A;;FA;;;SY)(A;;FA;;;BA)" + extra)
	if e != nil {
		t.Fatal(e)
	}
	acl, _, e := descriptor.DACL()
	if e != nil {
		t.Fatal(e)
	}
	if e = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); e != nil {
		t.Fatal(e)
	}
}

func TestRound2WindowsDoctorRejectsReadableTokenDACL(t *testing.T) {
	for _, sid := range []string{"WD", "BU", "AU"} {
		t.Run(sid, func(t *testing.T) {
			isolate(t)
			path := filepath.Join(t.TempDir(), "token")
			if e := os.WriteFile(path, []byte("sbp_test"), 0600); e != nil {
				t.Fatal(e)
			}
			setTokenDACL(t, path, "(A;;FR;;;"+sid+")")
			detail, e := TokenCheck(path)
			if output.ExitCode(e) != 3 || !strings.Contains(output.AsError(e).Hint, "icacls") || !strings.Contains(detail, path) {
				t.Fatal(detail, e)
			}
		})
	}
}

func TestRound2WindowsDoctorAcceptsPrivateTokenDACL(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(path, []byte("sbp_test"), 0600); e != nil {
		t.Fatal(e)
	}
	setTokenDACL(t, path, "")
	if _, e := TokenCheck(path); e != nil {
		t.Fatal(e)
	}
}

func TestRound2WindowsDoctorIgnoresMetadataAndInheritOnly(t *testing.T) {
	for _, ace := range []string{"(A;;0x80;;;WD)", "(A;OIIO;FR;;;WD)"} {
		t.Run(ace, func(t *testing.T) {
			isolate(t)
			path := filepath.Join(t.TempDir(), "token")
			if e := os.WriteFile(path, []byte("sbp_test"), 0600); e != nil {
				t.Fatal(e)
			}
			setTokenDACL(t, path, ace)
			if _, e := TokenCheck(path); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestRound2WindowsDoctorRejectsInheritedRead(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	setTokenDACL(t, dir, "(A;OICI;FR;;;BU)")
	path := filepath.Join(dir, "token")
	if e := os.WriteFile(path, []byte("sbp_test"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := TokenCheck(path); output.ExitCode(e) != 3 || !strings.Contains(output.AsError(e).Hint, "icacls") {
		t.Fatal(e)
	}
}

func TestRound2WindowsDoctorRejectsNullDACL(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(path, []byte("sbp_test"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, nil, nil); e != nil {
		t.Fatal(e)
	}
	if _, e := TokenCheck(path); output.ExitCode(e) != 3 || !strings.Contains(output.AsError(e).Hint, "icacls") {
		t.Fatal(e)
	}
}
