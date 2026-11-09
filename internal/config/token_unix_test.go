//go:build !windows

package config

import (
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenFIFORejectedPromptly(t *testing.T) {
	isolate(t)
	fifo := filepath.Join(t.TempDir(), "token.fifo")
	if e := unix.Mkfifo(fifo, 0600); e != nil {
		t.Fatal(e)
	}
	link := fifo + ".link"
	if e := os.Symlink(fifo, link); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{fifo, link} {
		done := make(chan error, 1)
		go func() { _, e := TokenFile(path); done <- e }()
		select {
		case e := <-done:
			if output.ExitCode(e) != 3 {
				t.Fatal(e)
			}
		case <-time.After(300 * time.Millisecond):
			// Release the old blocking implementation before reporting its failure.
			f, e := os.OpenFile(fifo, os.O_RDWR|unix.O_NONBLOCK, 0600)
			if e == nil {
				defer f.Close()
			}
			<-done
			t.Fatal("token FIFO read blocked")
		}
	}
}
