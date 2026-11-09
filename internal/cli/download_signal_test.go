package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDownloadSignalHelper(t *testing.T) {
	if os.Getenv("SBP_DOWNLOAD_SIGNAL_HELPER") != "1" {
		return
	}
	os.Exit(Run([]string{"download", "/api/invoices/1/pdf", "-o", os.Getenv("SBP_DOWNLOAD_TARGET"), "--json"}, os.Stdout, os.Stderr, Build{}))
}

func TestDownloadInterruptCleansStaging(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support os.Interrupt delivery")
	}
	downloadEnv(t)
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	dir := t.TempDir()
	target := filepath.Join(dir, "out.pdf")
	cmd := exec.Command(os.Args[0], "-test.run=^TestDownloadSignalHelper$")
	cmd.Env = append(os.Environ(), "SBP_DOWNLOAD_SIGNAL_HELPER=1", "SBP_DOWNLOAD_TARGET="+target, "SLOWBOOKS_URL="+server.URL)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("download did not start")
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
		<-done
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 8 {
			t.Errorf("exit: %v; %s", err, out.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("cancelled download did not stop")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal(fmt.Sprint(entries))
	}
}

func TestDownloadBrokenStdoutPipeProcess(t *testing.T) {
	downloadEnv(t)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	defer writer.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDownloadSignalHelper$")
	cmd.Env = append(os.Environ(), "SBP_DOWNLOAD_SIGNAL_HELPER=1", "SBP_DOWNLOAD_TARGET=-")
	cmd.Stdout = writer
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err = cmd.Run()
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 8 || !strings.Contains(stderr.String(), `"code":"network"`) {
		t.Fatal(err, stderr.String())
	}
}
