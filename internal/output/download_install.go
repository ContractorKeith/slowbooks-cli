package output

import (
	"context"
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
