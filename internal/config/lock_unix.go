//go:build !windows

package config

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryLock(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func unlock(f *os.File) error  { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
func lockBusy(e error) bool    { return errors.Is(e, unix.EWOULDBLOCK) || errors.Is(e, unix.EAGAIN) }
