package output

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var stagingName = regexp.MustCompile(`^\.sbp-download-[0-9]+$`)

// Cleanup is best effort: an unreadable or busy orphan must not block a download.
func cleanupDownloads(dir, destination string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, entry := range entries {
		// Preserve the requested target, including case-insensitive aliases.
		if strings.EqualFold(entry.Name(), destination) || !stagingName.MatchString(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		removeStaleDownload(filepath.Join(dir, entry.Name()), cutoff)
	}
}
