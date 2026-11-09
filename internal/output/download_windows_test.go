package output

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsDownloadInUse(t *testing.T) {
	p := filepath.Join(t.TempDir(), "saved")
	if e := os.WriteFile(p, []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	ptr, _ := windows.UTF16PtrFromString(p)
	h, e := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer windows.CloseHandle(h)
	f, e := NewDownloadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Abort()
	_, _ = f.Write([]byte("new"))
	_, e = f.Save(context.Background(), "", true)
	if e == nil || !strings.Contains(e.Error(), "file is in use") {
		t.Fatal(e)
	}
}
