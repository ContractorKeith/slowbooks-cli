//go:build !windows

package config

import (
	"fmt"
	"os"
)

func tokenPermissions(path string, mode os.FileMode) (string, error) {
	return fmt.Sprintf("%s (%04o)", path, mode.Perm()), nil
}
