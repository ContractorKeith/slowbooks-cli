package output

import (
	"io"
	"os"
	"path/filepath"
)

// DownloadStream reserves stdout for bytes, routing all diagnostics to stderr.
// The returned writer remains the original stdout destination.
func (w *Writer) DownloadStream() io.Writer {
	raw := w.Out
	w.Out = w.Err
	return struct{ io.Writer }{raw}
}

// DownloadFile stages bytes beside the destination and installs them only after
// the caller has received the complete response.
type DownloadFile struct {
	*os.File
	destination string
}

func NewDownloadFile(destination string) (*DownloadFile, error) {
	dir := "."
	if destination != "" {
		dir = filepath.Dir(destination)
	}
	file, err := os.CreateTemp(dir, ".sbp-download-*")
	if err != nil {
		return nil, err
	}
	return &DownloadFile{file, destination}, nil
}

// Abort releases the staging file, including after successful installation.
func (f *DownloadFile) Abort() { _ = f.Close(); _ = os.Remove(f.Name()) }

// Save uses no-clobber linking by default; force replaces the target by rename.
func (f *DownloadFile) Save(filename string, force bool) (string, error) {
	target := f.destination
	if target == "" {
		target = filename
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	var err error
	if force {
		err = os.Rename(f.Name(), target)
	} else {
		err = os.Link(f.Name(), target)
	}
	if os.IsExist(err) {
		return "", &Error{Code: "usage", Message: "destination exists", Hint: "use --force to overwrite"}
	}
	return target, err
}
