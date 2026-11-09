package output

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestDownloadWithoutHardLinks(t *testing.T) {
	for _, cause := range []error{syscall.EPERM, syscall.ENOTSUP, syscall.EXDEV, syscall.Errno(1)} {
		t.Run(cause.Error(), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "out.pdf")
			f, err := NewDownloadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Abort()
			f.link = func(old, new string) error { return &os.LinkError{Op: "link", Old: old, New: new, Err: cause} }
			if _, err := f.Write([]byte("complete")); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Save(context.Background(), "ignored", false); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "complete" {
				t.Fatal(string(data), err)
			}
			// Fallback must preserve no-clobber semantics too.
			g, err := NewDownloadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Abort()
			g.link = f.link
			_, _ = g.Write([]byte("replacement"))
			if _, err := g.Save(context.Background(), "ignored", false); ExitCode(err) != 2 {
				t.Fatal(err)
			}
			data, _ = os.ReadFile(target)
			if string(data) != "complete" {
				t.Fatal(string(data))
			}
		})
	}
}

func TestDownloadedFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions")
	}
	for _, force := range []bool{false, true} {
		target := filepath.Join(t.TempDir(), "out.pdf")
		f, err := NewDownloadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Abort()
		_, _ = f.Write([]byte("pdf"))
		if _, err := f.Save(context.Background(), "ignored", force); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0644 {
			t.Errorf("force=%v permissions=%o", force, info.Mode().Perm())
		}
	}
}

func TestDownloadCancelledBeforeInstall(t *testing.T) {
	target := filepath.Join(t.TempDir(), "out.pdf")
	f, err := NewDownloadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Abort()
	_, _ = f.Write([]byte("complete"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Save(ctx, "ignored", false); ExitCode(err) != 8 {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestFailedFallbackRemovesPartialTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "out.pdf")
	// Reading a directory fails after O_EXCL creates the target.
	err := copyExclusive(context.Background(), t.TempDir(), target)
	if err == nil {
		t.Fatal("copy unexpectedly succeeded")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
