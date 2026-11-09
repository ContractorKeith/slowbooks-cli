package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"golang.org/x/sys/windows"
)

func TestRound2WindowsConfigShare(t *testing.T) {
	share := os.Getenv("SBP_FIXB_SHARE")
	if share == "" {
		t.Skip("requires Parallels share")
	}
	isolate(t)
	dir, e := os.MkdirTemp(share, "config-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	t.Setenv("SLOWBOOKS_CONFIG_DIR", dir)
	for _, token := range []string{"sbp_first", "sbp_second"} {
		if e = SaveLogin("a", Profile{}, token); e != nil {
			t.Fatal(e)
		}
	}
	if e = Logout("a"); e != nil {
		t.Fatal(e)
	}
	s, e := Load()
	if e != nil || len(s.Profiles) != 0 {
		t.Fatal(s, e)
	}
}

func holdWithoutDelete(t *testing.T, path string) windows.Handle {
	t.Helper()
	ptr, e := windows.UTF16PtrFromString(path)
	if e != nil {
		t.Fatal(e)
	}
	h, e := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	return h
}

func TestRound2WindowsRenameWaitsForOtherProgram(t *testing.T) {
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
			h := holdWithoutDelete(t, path)
			done := make(chan error, 1)
			go func() { done <- SaveLogin("a", Profile{}, "sbp_new") }()
			time.Sleep(650 * time.Millisecond)
			_ = windows.CloseHandle(h)
			if e := <-done; e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestRound2WindowsRenameBusyDiagnostic(t *testing.T) {
	isolate(t)
	if e := SaveLogin("a", Profile{}, "sbp_old"); e != nil {
		t.Fatal(e)
	}
	path, _ := Path()
	h := holdWithoutDelete(t, path)
	defer windows.CloseHandle(h)
	start := time.Now()
	e := SaveLogin("a", Profile{}, "sbp_new")
	elapsed := time.Since(start)
	if output.ExitCode(e) != 3 || output.AsError(e).Code != "auth" || !strings.Contains(e.Error(), "in use by another program: "+path) {
		t.Fatal(e)
	}
	if elapsed < 1500*time.Millisecond || elapsed > 4*time.Second {
		t.Fatal("retry duration", elapsed)
	}
}

func TestRound2WindowsReadOnlyIsNotReportedAsHeld(t *testing.T) {
	isolate(t)
	if e := SaveLogin("a", Profile{}, "sbp_old"); e != nil {
		t.Fatal(e)
	}
	path, _ := Path()
	if e := os.Chmod(path, 0444); e != nil {
		t.Fatal(e)
	}
	defer os.Chmod(path, 0600)
	e := SaveLogin("a", Profile{}, "sbp_new")
	if output.ExitCode(e) != 3 || !strings.Contains(e.Error(), path) {
		t.Fatal(e)
	}
	if strings.Contains(e.Error(), "in use by another program") {
		t.Fatal("read-only file misreported as a held handle", e)
	}
}
