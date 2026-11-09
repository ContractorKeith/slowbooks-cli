package output

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadCleansOnlyOldStagingFiles(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, ".sbp-download-old")
	fresh := filepath.Join(dir, ".sbp-download-fresh")
	other := filepath.Join(dir, "unrelated")
	oldTime := time.Now().Add(-2 * time.Hour)
	for _, path := range []string{old, fresh, other} {
		if err := writeOwnedDownloadFixture(path, []byte("keep bytes")); err != nil {
			t.Fatal(err)
		}
		if path != fresh {
			if err := os.Chtimes(path, oldTime, oldTime); err != nil {
				t.Fatal(err)
			}
		}
	}
	nested := filepath.Join(dir, ".sbp-download-directory")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(nested, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	f, err := NewDownloadFile(filepath.Join(dir, "result"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if _, err = os.Lstat(old); !os.IsNotExist(err) {
		t.Fatalf("old staging file remains: %v", err)
	}
	for _, path := range []string{fresh, other, nested} {
		if _, err = os.Lstat(path); err != nil {
			t.Fatal("removed protected entry", path, err)
		}
	}
}

func TestDownloadCleanupPreservesSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(target, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".sbp-download-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	f, err := NewDownloadFile(filepath.Join(dir, "result"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink changed", info, err)
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "original" {
		t.Fatal("target changed", string(b), err)
	}
}
