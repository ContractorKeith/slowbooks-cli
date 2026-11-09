//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"golang.org/x/sys/unix"
)

func TestRecoveryPreservesFIFO(t *testing.T) {
	isolate(t)
	path, _ := Path()
	if e := unix.Mkfifo(path, 0600); e != nil {
		t.Fatal(e)
	}
	_, _, e := LoginProfile(Flags{})
	if output.ExitCode(e) != 3 || !strings.Contains(e.Error(), path) {
		t.Fatal(e)
	}
	if info, e := os.Lstat(path); e != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatal("changed FIFO", info, e)
	}
	backups, _ := filepath.Glob(path + ".corrupt-*")
	if len(backups) != 0 {
		t.Fatal(backups)
	}
}
