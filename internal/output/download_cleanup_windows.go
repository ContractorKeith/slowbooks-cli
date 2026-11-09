package output

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func removeStaleDownload(path string, cutoff time.Time) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	// Open the entry itself. Deletion uses this verified handle, not a second path lookup.
	h, err := windows.CreateFile(ptr, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES|windows.DELETE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(h, &info) != nil || info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return
	}
	kind, err := windows.GetFileType(h)
	if err != nil || kind != windows.FILE_TYPE_DISK || !time.Unix(0, info.LastWriteTime.Nanoseconds()).Before(cutoff) {
		return
	}
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || !owner.Equals(user.User.Sid) {
		return
	}
	// FILE_DISPOSITION_INFO contains one Win32 BOOL: mark only this handle for deletion.
	disposition := [4]byte{1, 0, 0, 0}
	_ = windows.SetFileInformationByHandle(h, windows.FileDispositionInfo, &disposition[0], uint32(len(disposition)))
}

// CreateTemp's live write handle already prevents an exclusive cleanup open.
func lockDownload(*os.File) error { return nil }
