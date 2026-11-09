package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

// A reader or scanner may briefly hold the destination without share-delete.
func renameFile(old, target string) error {
	// Root.Rename uses Windows POSIX replacement semantics, so share-delete
	// readers keep their old handles while new readers see the replacement.
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer root.Close()
	source, err := filepath.Rel(filepath.Dir(target), old)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 8; attempt++ {
		err = root.Rename(source, filepath.Base(target))
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		if attempt < 7 {
			time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
		}
	}
	return err
}
