package output

import (
	"errors"
	"golang.org/x/sys/windows"
)

func downloadInUse(target string, err error) bool {
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return true
	}
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return false
	}
	// MoveFileEx can report access denied for a shared target. Probe delete access
	// to distinguish that case from an ACL or read-only-file denial.
	ptr, e := windows.UTF16PtrFromString(target)
	if e != nil {
		return false
	}
	h, e := windows.CreateFile(ptr, windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e == nil {
		_ = windows.CloseHandle(h)
	}
	return errors.Is(e, windows.ERROR_SHARING_VIOLATION)
}
