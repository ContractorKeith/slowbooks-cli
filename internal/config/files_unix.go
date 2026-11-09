//go:build !windows

package config

import "os"

func renameFile(old, target string) error { return os.Rename(old, target) }
