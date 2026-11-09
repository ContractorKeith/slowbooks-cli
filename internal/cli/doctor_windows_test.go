package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/config"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"golang.org/x/sys/windows"
)

// Doctor's success fixtures need a private parent, rather than inheriting the
// broad ACL of a runner's shared temporary directory or C:\ scratch directory.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "slowbooks-doctor-tests-")
	if err == nil {
		err = setDoctorFixtureACL(dir, "")
	}
	if err != nil {
		if dir != "" {
			_ = os.RemoveAll(dir)
		}
		_ = output.New(os.Stderr, os.Stderr).Text("prepare private doctor test directory: " + err.Error() + "\n")
		os.Exit(1)
	}
	_ = os.Setenv("TMP", dir)
	_ = os.Setenv("TEMP", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func setDoctorFixtureACL(path, extra string) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + owner.String() + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)" + extra)
	if err != nil {
		return err
	}
	acl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

func TestRound2DoctorTokenDACLRow(t *testing.T) {
	_, url := setupAuth(t)
	if code, out, _ := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", url); code != 0 {
		t.Fatal(code, out)
	}
	s, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	path := s.Profiles[s.Default].TokenFile
	if err = setDoctorFixtureACL(path, "(A;;FR;;;WD)"); err != nil {
		t.Fatal(err)
	}
	code, out, _ := invoke(t, "", "doctor", "--json")
	var result struct {
		Data struct {
			Checks []struct {
				Name   string
				OK     bool
				Detail string
			}
		}
	}
	if code != 3 || json.Unmarshal([]byte(out), &result) != nil {
		t.Fatal(code, out)
	}
	for _, row := range result.Data.Checks {
		if row.Name == "token file" {
			if row.OK || !strings.Contains(row.Detail, "icacls") {
				t.Fatal(row)
			}
			return
		}
	}
	t.Fatal("missing token check")
}
