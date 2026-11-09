package output

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSweepPreservesPrefixUserFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, ".sbp-download-keep.pdf")
	if err := writeOwnedDownloadFixture(path, []byte("user file")); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	f, err := NewDownloadFile("")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if b, err := os.ReadFile(path); err != nil || string(b) != "user file" {
		t.Fatal("sweep changed user file", string(b), err)
	}
}

func TestSweepPreservesRecentStaging(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".sbp-download-77777")
	if err := writeOwnedDownloadFixture(path, []byte("recent")); err != nil {
		t.Fatal(err)
	}
	age := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, age, age); err != nil {
		t.Fatal(err)
	}
	f, err := NewDownloadFile(filepath.Join(dir, "result"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("removed a staging file younger than 24 hours", err)
	}
}

func TestSweepPreservesActiveStagingWriter(t *testing.T) {
	dir := t.TempDir()
	f, err := NewDownloadFile(filepath.Join(dir, "first"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	if _, err = f.Write([]byte("partial")); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err = os.Chtimes(f.file.Name(), old, old); err != nil {
		t.Fatal(err)
	}
	next, err := NewDownloadFile(filepath.Join(dir, "second"))
	if err != nil {
		t.Fatal(err)
	}
	defer next.Abort()
	if _, err = os.Lstat(f.file.Name()); err != nil {
		t.Fatal("removed an active staging writer", err)
	}
}

func TestFallbackRetainsLinkFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "missing", "saved")
	f, err := NewDownloadFile(filepath.Join(dir, "initial"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	// The target parent disappears after staging; both link and copying fail.
	f.destination = target
	f.link = func(string, string) error {
		return &os.LinkError{Op: "link", Old: f.file.Name(), New: target, Err: errors.New("original link failure")}
	}
	_, _ = f.Write([]byte("body"))
	_, err = f.Save(context.Background(), "", false)
	if err == nil || !strings.Contains(err.Error(), "original link failure") {
		t.Fatal(err)
	}
	detail := AsError(err).Detail
	text, ok := detail.(string)
	if !ok || !strings.Contains(text, "copy") || !strings.Contains(text, strconv.Quote(target)) || strings.Contains(text, filepath.Base(f.file.Name())) {
		t.Fatal("missing or unsafe copy detail", detail)
	}
}
