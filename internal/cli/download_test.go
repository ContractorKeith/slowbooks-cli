package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadFilesAndStdout(t *testing.T) {
	_, server := setupAuth(t)
	t.Setenv("SLOWBOOKS_URL", server)
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("sbp_test"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLOWBOOKS_TOKEN_FILE", token)
	dir := t.TempDir()
	t.Chdir(dir)
	code, out, _ := invoke(t, "", "invoices", "pdf", "1")
	if code != 0 {
		t.Fatal(code, out)
	}
	data, err := os.ReadFile("invoice-1.pdf")
	if err != nil || !strings.HasPrefix(string(data), "%PDF-1.4") {
		t.Fatal(string(data), err)
	}
	code, out, _ = invoke(t, "", "invoices", "pdf", "1")
	if code != 2 {
		t.Fatal(code, out)
	}
	code, out, _ = invoke(t, "", "invoices", "pdf", "1", "--force")
	if code != 0 {
		t.Fatal(code, out)
	}
	code, out, _ = invoke(t, "", "download", "/api/invoices/traversal/pdf")
	if code != 0 {
		t.Fatal(code, out)
	}
	if _, err := os.Stat("evil.pdf"); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := invoke(t, "", "download", "/api/invoices/1/pdf", "-o", "-")
	if code != 0 || out != string(data) || stderr != "" {
		t.Fatal(code, out, stderr)
	}
	code, out, stderr = invoke(t, "", "invoices", "pdf", "1", "-o", "-", "--json")
	var envelope struct {
		Data downloadData `json:"data"`
	}
	if code != 0 || out != string(data) || json.Unmarshal([]byte(stderr), &envelope) != nil || envelope.Data.Path != "-" || envelope.Data.Bytes != int64(len(data)) {
		t.Fatal(code, out, stderr)
	}
	code, out, stderr = invoke(t, "", "download", "/api/not-found", "-o", "-", "--json")
	if code != 5 || out != "" || !strings.Contains(stderr, `"not_found"`) {
		t.Fatal(code, out, stderr)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".slowbooks-download-") {
			t.Fatal(e.Name())
		}
	}
}

func TestFailedDownloadPreservesDestination(t *testing.T) {
	isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("partial"))
	}))
	defer server.Close()
	t.Setenv("SLOWBOOKS_URL", server.URL)
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("sbp_test"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLOWBOOKS_TOKEN_FILE", token)
	dir := t.TempDir()
	target := filepath.Join(dir, "saved")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := invoke(t, "", "download", "/api/file", "-o", target, "--force")
	data, _ := os.ReadFile(target)
	entries, _ := os.ReadDir(dir)
	if code != 8 || string(data) != "original" || len(entries) != 1 {
		t.Fatal(code, out, string(data), entries)
	}
}

func TestSafeFilename(t *testing.T) {
	for input, want := range map[string]string{"../../evil.pdf": "evil.pdf", `..\..\evil.pdf`: "evil.pdf", ".hidden": "hidden", "a..b\x00\n.pdf": "ab.pdf", "..": "", "/": ""} {
		if got := safeFilename(input); got != want {
			t.Errorf("%q: %q want %q", input, got, want)
		}
	}
}
