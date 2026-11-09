package output

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"syscall"
)

func unsupportedLink(err error) bool {
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EXDEV) {
		return true
	}
	// Windows filesystems may report invalid function, not supported, or a
	// different device using Win32 codes instead of Unix errno aliases.
	return runtime.GOOS == "windows" && (errors.Is(err, syscall.Errno(1)) || errors.Is(err, syscall.Errno(50)) || errors.Is(err, syscall.Errno(17)))
}

// copyExclusive supports filesystems without hard links. A failed copy removes
// the newly created target, and O_EXCL preserves any existing destination.
func copyExclusive(ctx context.Context, source, target string) (err error) {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(target)
		}
	}()
	_, err = io.Copy(dst, downloadCopyReader{ctx, src})
	closeErr := dst.Close()
	if err == nil {
		err = closeErr
	}
	return err
}

// Check cancellation between disk reads during fallback installation too.
type downloadCopyReader struct {
	ctx context.Context
	io.Reader
}

func (r downloadCopyReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}
