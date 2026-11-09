package output

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cleanup is best effort: an unreadable or busy orphan must not block a download.
func cleanupDownloads(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-time.Hour)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".sbp-download-") || !entry.Type().IsRegular() {
			continue
		}
		removeStaleDownload(filepath.Join(dir, entry.Name()), cutoff)
	}
}
