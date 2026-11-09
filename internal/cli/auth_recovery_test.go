package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/config"
)

func TestAuthLoginReportsCorruptProfileBackup(t *testing.T) {
	_, url := setupAuth(t)
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"profiles":`)
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := invoke(t, "sbp_bookkeeper", "auth", "login", "--url", url)
	if code != 0 {
		t.Fatal(code, out, stderr)
	}
	backups, err := filepath.Glob(path + ".corrupt-*")
	if err != nil || len(backups) != 1 {
		t.Fatal(backups, err)
	}
	if !strings.Contains(stderr, backups[0]) {
		t.Fatal("backup not reported", stderr)
	}
	if b, err := os.ReadFile(backups[0]); err != nil || string(b) != string(raw) {
		t.Fatal(string(b), err)
	}
}
