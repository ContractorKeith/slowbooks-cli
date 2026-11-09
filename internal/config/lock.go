package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

// Kernel locks serialize mutations across processes and release on process death.
// Keep the lock file: unlinking it can create two independent lock owners.
func lockStore() (func(), error) {
	dir, e := Dir()
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return nil, fileError("prepare config directory", dir, e)
	}
	path := filepath.Join(dir, ".profiles.lock")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, fileError("open config lock", path, e)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		e = tryLock(f)
		if e == nil {
			return func() { _ = unlock(f); _ = f.Close() }, nil
		}
		if !lockBusy(e) {
			_ = f.Close()
			return nil, fileError("lock config", path, e)
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, &output.Error{Code: "auth", Message: "config is busy: " + path, Hint: "retry after the other sbp process finishes"}
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func fileError(action, path string, cause error) error {
	// Retain the OS reason in a typed error without leaking credential content.
	var pathErr *os.PathError
	if errors.As(cause, &pathErr) {
		cause = pathErr.Err
	}
	return &output.Error{Code: "auth", Message: "cannot " + action + ": " + path + ": " + cause.Error(), Hint: "check the path and permissions"}
}
