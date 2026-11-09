package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsReadersPermitReplacement(t *testing.T) {
	isolate(t)
	if e := SaveLogin("a", Profile{}, "sbp_first"); e != nil {
		t.Fatal(e)
	}
	p, _ := Path()
	s, _ := Load()
	for _, path := range []string{p, s.Profiles["a"].TokenFile} {
		f, e := openToken(path)
		if e != nil {
			t.Fatal(e)
		}
		replacement := filepath.Join(filepath.Dir(path), "replacement")
		if e = os.WriteFile(replacement, []byte("sbp_second"), 0600); e != nil {
			t.Fatal(e)
		}
		e = renameFile(replacement, path)
		b, readErr := io.ReadAll(f)
		_ = f.Close()
		if readErr != nil || len(b) == 0 || string(b) == "sbp_second" {
			t.Fatal("reader lost original file", string(b), readErr)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
}
func TestWindowsRenameRetriesSharingViolation(t *testing.T) {
	isolate(t)
	if e := SaveLogin("a", Profile{}, "sbp_first"); e != nil {
		t.Fatal(e)
	}
	p, _ := Path()
	s, _ := Load()
	for _, path := range []string{p, s.Profiles["a"].TokenFile} {
		ptr, e := windows.UTF16PtrFromString(path)
		if e != nil {
			t.Fatal(e)
		}
		h, e := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		if e != nil {
			t.Fatal(e)
		}
		done := make(chan error, 1)
		go func() { done <- SaveLogin("a", Profile{}, "sbp_second") }()
		time.Sleep(80 * time.Millisecond)
		_ = windows.CloseHandle(h)
		if e = <-done; e != nil {
			t.Fatal(e)
		}
	}
}

func TestWindowsTokenDoctorACL(t *testing.T) {
	isolate(t)
	if e := SaveLogin("a", Profile{}, "sbp_test"); e != nil {
		t.Fatal(e)
	}
	s, _ := Load()
	path := s.Profiles["a"].TokenFile
	setTokenDACL(t, path, "")
	detail, e := TokenCheck(path)
	if e != nil || !strings.Contains(detail, "DACL checked") || strings.Contains(detail, "0666") {
		t.Fatal(detail, e)
	}
}
