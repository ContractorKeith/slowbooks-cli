package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func downloadEnv(t *testing.T) {
	t.Helper()
	_, server := setupAuth(t)
	t.Setenv("SLOWBOOKS_URL", server)
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("sbp_test"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLOWBOOKS_TOKEN_FILE", token)
}

func TestInvoicePDFRejectsQueryAndFragmentIDs(t *testing.T) {
	downloadEnv(t)
	for _, id := range []string{"1?x=", "1#fragment", "", "1/2"} {
		code, out, stderr := invoke(t, "", "invoices", "pdf", id, "-o", "-")
		if code != 2 || out != "" {
			t.Errorf("%q: exit %d stdout %s stderr %s", id, code, out, stderr)
		}
	}
}

func TestRawDownloadEarlyErrorsUseStderr(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{
		{"download", "-o", "-", "--json"},
		{"download", "/api/file", "-o", "-", "--json", "--unknown"},
		{"invoices", "pdf", "-o-", "--json"},
		{"download", "/api/file", "--output=-", "--unknown", "--json"},
	} {
		code, out, stderr := invoke(t, "", args...)
		if code != 2 || out != "" || !strings.Contains(stderr, `"code":"usage"`) {
			t.Errorf("%v: exit %d stdout %s stderr %s", args, code, out, stderr)
		}
	}
}

func TestDownloadDestinationErrorsAreCleanUsage(t *testing.T) {
	downloadEnv(t)
	dir := t.TempDir()
	for _, target := range []string{filepath.Join(dir, "nodir", "out.pdf"), dir} {
		code, out, _ := invoke(t, "", "download", "/api/invoices/1/pdf", "-o", target, "--force")
		var envelope struct {
			Error struct{ Code, Message string }
		}
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatal(err)
		}
		if code != 2 || envelope.Error.Code != "usage" || !strings.Contains(envelope.Error.Message, strconv.Quote(target)) || strings.Contains(out, ".slowbooks-download-") {
			t.Errorf("%s: exit %d %s", target, code, out)
		}
	}
}

type closedDownloadPipe struct{}

func (closedDownloadPipe) Write([]byte) (int, error) {
	return 0, &os.PathError{Op: "write", Path: "stdout", Err: syscall.EPIPE}
}

func TestRawDownloadBrokenPipeIsExitEight(t *testing.T) {
	downloadEnv(t)
	var stderr bytes.Buffer
	code := Run([]string{"download", "/api/invoices/1/pdf", "-o", "-", "--json"}, closedDownloadPipe{}, &stderr, Build{})
	if code != 8 || !strings.Contains(stderr.String(), `"code":"network"`) || !strings.Contains(stderr.String(), `"retryable":false`) {
		t.Fatal(code, stderr.String())
	}
}

func TestInvoicePDFRejectsParamButKeepsForce(t *testing.T) {
	downloadEnv(t)
	code, out, stderr := invoke(t, "", "invoices", "pdf", "1", "-o", "-", "--param", "q=value")
	if code != 2 || out != "" || !strings.Contains(stderr, "unknown flag") {
		t.Fatal(code, out, stderr)
	}
	target := filepath.Join(t.TempDir(), "out.pdf")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr = invoke(t, "", "invoices", "pdf", "1", "-o", target, "--force")
	data, _ := os.ReadFile(target)
	if code != 0 || !strings.HasPrefix(string(data), "%PDF") {
		t.Fatal(code, out, stderr, string(data))
	}
}

func TestRawDownloadEarlyTerminalJSON(t *testing.T) {
	isolate(t)
	var out, stderr bytes.Buffer
	w := output.New(&out, &stderr)
	w.TTY = true
	root := NewRoot(w, Build{})
	code := Execute(root, w, []string{"--profile", "unused", "download", "-o", "-", "--unknown", "--json"})
	if code != 2 || out.Len() != 0 || !json.Valid(stderr.Bytes()) {
		t.Fatal(code, out.String(), stderr.String())
	}
}
