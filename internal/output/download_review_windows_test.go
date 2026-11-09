package output

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestSweepPreservesSharedReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".sbp-download-88888")
	if err := writeOwnedDownloadFixture(path, []byte("open")); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	ptr, _ := windows.UTF16PtrFromString(path)
	h, err := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	f, err := NewDownloadFile(filepath.Join(dir, "result"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if _, err = os.Stat(path); err != nil {
		t.Fatal("swept a file opened by another reader", err)
	}
}
