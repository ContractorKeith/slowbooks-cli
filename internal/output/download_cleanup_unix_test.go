//go:build !windows

package output

import "os"

func writeOwnedDownloadFixture(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
