//go:build !windows

package output

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func removeStaleDownload(path string, cutoff time.Time) {
	// Inspect without following links, then hold the same advisory lock as writers.
	initial, err := os.Lstat(path)
	if err != nil || !staleUserFile(initial, cutoff) {
		return
	}
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return
	}
	defer f.Close()
	if err = lockDownload(f); err != nil {
		return
	}
	info, err := f.Stat()
	if err != nil || !staleUserFile(info, cutoff) || !os.SameFile(initial, info) {
		return
	}
	// Recheck identity immediately before unlinking; Unlink never follows links or removes directories.
	current, err := os.Lstat(path)
	if err != nil || !staleUserFile(current, cutoff) || !os.SameFile(info, current) {
		return
	}
	_ = syscall.Unlink(path)
}

func staleUserFile(info os.FileInfo, cutoff time.Time) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid()) && info.Mode().IsRegular() && info.ModTime().Before(cutoff)
}

// Held until the staging descriptor closes, including during writes and rewinds.
func lockDownload(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
