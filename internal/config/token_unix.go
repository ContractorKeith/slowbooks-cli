//go:build !windows

package config

import (
	"os"

	"golang.org/x/sys/unix"
)

func openToken(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}
