package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Explicit opt-in keeps CI independent of a particular network filesystem.
func TestDownloadToNoHardLinkShare(t *testing.T) {
	share := os.Getenv("SBP_FIXB_SHARE")
	if share == "" {
		t.Skip("requires no-hard-link share")
	}
	_, url := setupAuth(t)
	t.Setenv("SLOWBOOKS_URL", url)
	token := filepath.Join(t.TempDir(), "token")
	if e := os.WriteFile(token, []byte("sbp_test"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("SLOWBOOKS_TOKEN_FILE", token)
	dir, e := os.MkdirTemp(share, "download-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	target := filepath.Join(dir, "saved.pdf")
	code, out, _ := invoke(t, "", "invoices", "pdf", "1", "-o", target)
	if code != 0 {
		t.Fatal(code, out)
	}
	b, e := os.ReadFile(target)
	if e != nil || !strings.HasPrefix(string(b), "%PDF-") {
		t.Fatal(string(b), e)
	}
}
