package output

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// DownloadStream reserves stdout for bytes, routing all diagnostics to stderr.
// The returned writer remains the original stdout destination.
func (w *Writer) DownloadStream() io.Writer {
	if w.raw == nil {
		w.raw = downloadStream{w.Out}
	}
	w.Out = w.Err
	return w.raw
}

// DownloadFile stages bytes beside the destination and installs them only after
// the caller has received the complete response.
type DownloadFile struct {
	file        *os.File
	destination string
	link        func(string, string) error
}

// NewDownloadFile creates private staging beside the requested destination.
func NewDownloadFile(destination string) (*DownloadFile, error) {
	dir := "."
	if destination != "" {
		dir = filepath.Dir(destination)
	}
	cleanupDownloads(dir, filepath.Base(destination))
	file, err := os.CreateTemp(dir, ".sbp-download-*")
	if err != nil {
		return nil, destinationError(destination, err)
	}
	return &DownloadFile{file: file, destination: destination, link: os.Link}, nil
}

// Abort releases the staging file, including after successful installation.
func (f *DownloadFile) Abort() { _ = f.file.Close(); _ = os.Remove(f.file.Name()) }

// Save uses no-clobber linking by default; force replaces the target by rename.
func (f *DownloadFile) Save(ctx context.Context, filename string, force bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", destinationError(f.destination, err)
	}
	target := f.destination
	if target == "" {
		target = filename
	}
	if err := f.file.Chmod(0644); err != nil {
		return "", destinationError(target, err)
	}
	if err := f.file.Close(); err != nil {
		return "", destinationError(target, err)
	}
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return "", &Error{Code: "usage", Message: fmt.Sprintf("destination %q is a directory", target)}
	}
	var err error
	if force {
		err = os.Rename(f.file.Name(), target)
	} else {
		err = f.link(f.file.Name(), target)
		if err != nil && !os.IsExist(err) {
			err = copyExclusive(ctx, f.file.Name(), target)
		}
	}
	if os.IsExist(err) {
		return "", &Error{Code: "usage", Message: fmt.Sprintf("destination %q exists", target), Hint: "use --force to overwrite"}
	}
	if err != nil {
		return "", destinationError(target, err)
	}
	return target, nil
}

func destinationError(target string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: "network", Message: "download cancelled", Retryable: false}
	}
	if downloadInUse(target, err) {
		return &Error{Code: "usage", Message: fmt.Sprintf("cannot save download to %q: file is in use", target), Hint: "close the file in the other process and retry"}
	}
	if target == "" {
		target = "current directory"
	}
	if e, ok := err.(*os.PathError); ok {
		err = e.Err
	}
	if e, ok := err.(*os.LinkError); ok {
		err = e.Err
	}
	return &Error{Code: "usage", Message: fmt.Sprintf("cannot save download to %q: %s", target, err)}
}

func (f *DownloadFile) Write(p []byte) (int, error) {
	n, err := f.file.Write(p)
	return n, destinationError(f.destination, err)
}
func (f *DownloadFile) Seek(offset int64, whence int) (int64, error) {
	n, err := f.file.Seek(offset, whence)
	return n, destinationError(f.destination, err)
}
func (f *DownloadFile) Truncate(size int64) error {
	return destinationError(f.destination, f.file.Truncate(size))
}

// downloadStream hides seeking and maps pipe failures without exposing OS paths.
type downloadStream struct{ io.Writer }

func (s downloadStream) Write(p []byte) (int, error) {
	n, err := s.Writer.Write(p)
	if err != nil {
		return n, &Error{Code: "network", Message: "could not write download stream", Retryable: false}
	}
	return n, nil
}
