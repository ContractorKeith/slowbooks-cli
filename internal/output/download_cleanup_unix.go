//go:build !windows

package output

import (
	"os"
	"syscall"
	"time"
)

func removeStaleDownload(path string, cutoff time.Time) {
	// Lstat inspects the entry itself, including when it has become a symlink.
	info, err := os.Lstat(path)
	if err != nil || !staleUserFile(info, cutoff) {
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
