package output

import (
	"context"
	"errors"
	"io"
	"os"
)

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

// Preserve the installation cause while reporting why the fallback also failed.
func fallbackError(target string, linkErr, copyErr error) error {
	if errors.Is(copyErr, context.Canceled) || errors.Is(copyErr, context.DeadlineExceeded) {
		return destinationError(target, copyErr)
	}
	failure := AsError(destinationError(target, linkErr))
	failure.Detail = "copy fallback: " + destinationError(target, copyErr).Error()
	if os.IsExist(copyErr) {
		failure.Hint = "use --force to overwrite"
	}
	return failure
}
