package config

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Query the file's volume, including UNC shares and mounted volumes, before
// interpreting a DACL that the filesystem may not persist or enforce.
func persistentACLs(path string) (bool, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	name, err := windows.UTF16PtrFromString(absolute)
	if err != nil {
		return false, err
	}
	root := make([]uint16, 32768)
	if err = windows.GetVolumePathName(name, &root[0], uint32(len(root))); err != nil {
		return false, err
	}
	var flags uint32
	if err = windows.GetVolumeInformation(&root[0], nil, 0, nil, nil, &flags, nil, 0); err != nil {
		return false, err
	}
	return flags&windows.FILE_PERSISTENT_ACLS != 0, nil
}
