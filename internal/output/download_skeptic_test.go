package output

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestLinkFailureFallback(t *testing.T) {
	for _, cause := range []error{syscall.Errno(87), syscall.EACCES} {
		target := filepath.Join(t.TempDir(), "saved")
		f, e := NewDownloadFile(target)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Abort()
		f.link = func(string, string) error { return cause }
		_, _ = f.Write([]byte("complete"))
		if _, e = f.Save(context.Background(), "", false); e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile(target)
		if e != nil || string(b) != "complete" {
			t.Fatal(string(b), e)
		}
	}
}
func TestDirectoryDestination(t *testing.T) {
	target := t.TempDir()
	f, e := NewDownloadFile(target)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Abort()
	_, e = f.Save(context.Background(), "", false)
	if e == nil || !strings.Contains(e.Error(), "is a directory") || AsError(e).Hint != "" {
		t.Fatal(e)
	}
}
