package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"golang.org/x/sys/windows"
)

// A reader or scanner may briefly hold the destination without share-delete.
func renameFile(old, target string) error {
	deadline := time.Now().Add(2 * time.Second)
	delay := 10 * time.Millisecond
	for {
		err := replaceFile(old, target)
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !fileHeld(old) && !fileHeld(target) {
				return err
			}
			return &output.Error{Code: "auth", Message: "in use by another program: " + target, Hint: "close programs using " + target + " and retry"}
		}
		time.Sleep(min(delay, remaining))
		delay = min(delay*2, 100*time.Millisecond)
	}
}

// Root.Rename preserves share-delete readers on NTFS. Unsupported filesystems
// use Go's MoveFileEx replacement instead, without unlinking the destination.
func replaceFile(old, target string) error {
	root, err := os.OpenRoot(filepath.Dir(target))
	if err == nil {
		defer root.Close()
		var source string
		source, err = filepath.Rel(filepath.Dir(target), old)
		if err == nil {
			err = root.Rename(source, filepath.Base(target))
		}
	}
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
		return os.Rename(old, target)
	}
	return err
}

// Access denied also covers read-only files and ACLs. Confirm a live competing
// handle with an attribute/delete-access exclusive probe before classifying it.
// The probe does not read data or delete the file.
func fileHeld(path string) bool {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	h, err := windows.CreateFile(ptr, windows.FILE_READ_ATTRIBUTES|windows.DELETE, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		_ = windows.CloseHandle(h)
	}
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
