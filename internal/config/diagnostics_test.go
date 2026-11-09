package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestMissingConnectionPiecesHaveSpecificErrors(t *testing.T) {
	isolate(t)
	t.Setenv("SLOWBOOKS_URL", "https://example.test")
	_, e := Resolve(Flags{})
	if output.ExitCode(e) != 3 || !strings.Contains(output.AsError(e).Hint, "SLOWBOOKS_TOKEN_FILE") {
		t.Fatal(e)
	}
	t.Setenv("SLOWBOOKS_URL", "")
	if e = SaveLogin("no-url", Profile{}, "sbp_test"); e != nil {
		t.Fatal(e)
	}
	_, e = Resolve(Flags{})
	if output.ExitCode(e) != 2 || !strings.Contains(e.Error(), "URL") {
		t.Fatal(e)
	}
}
func TestUnreadableStateErrorsIdentifyPaths(t *testing.T) {
	isolate(t)
	path, _ := Path()
	if e := os.MkdirAll(path, 0700); e != nil {
		t.Fatal(e)
	}
	_, e := Load()
	assertAuthPath(t, e, path)
	missingToken := filepath.Join(t.TempDir(), "missing.token")
	_, e = TokenFile(missingToken)
	assertAuthPath(t, e, missingToken)
}
func assertAuthPath(t *testing.T, e error, path string) {
	t.Helper()
	var typed *output.Error
	var pathErr *os.PathError
	if e == nil || output.ExitCode(e) != 3 || !errors.As(e, &typed) || errors.As(e, &pathErr) || !strings.Contains(e.Error(), path) {
		t.Fatalf("wanted auth error naming path: %T %v", e, e)
	}
}
func TestDoctorDetailOmitsEmptyHint(t *testing.T) {
	isolate(t)
	detail, e := TokenCheck(t.TempDir())
	if e == nil {
		t.Fatal("accepted directory")
	}
	if strings.HasSuffix(detail, "; ") {
		t.Fatalf("empty hint delimiter: %q", detail)
	}
}
