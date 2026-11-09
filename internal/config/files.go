package config

import (
	"os"
	"path/filepath"
)

// Stage private bytes on the same filesystem without altering an existing file.
func stageFile(path string, b []byte) (name string, err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".sbp-*")
	if err != nil {
		return "", err
	}
	name = f.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return name, err
}
func atomicWrite(path string, b []byte) error {
	name, e := stageFile(path, b)
	if e != nil {
		return e
	}
	defer os.Remove(name)
	return renameFile(name, path)
}
