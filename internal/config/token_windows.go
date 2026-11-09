package config

import (
	"os"

	"golang.org/x/sys/windows"
)

// Sharing deletion lets atomic profile and token replacements coexist with reads.
func openToken(path string) (*os.File, error) { return openShared(path, 0) }
func openProfiles(path string) (*os.File, error) {
	return openShared(path, windows.FILE_FLAG_OPEN_REPARSE_POINT)
}
func openShared(path string, flags uint32) (*os.File, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|flags, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
