package commands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
)

func TestManifestContract(t *testing.T) {
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
	m, err := Manifest("v1.2.3", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != 1 || !regexp.MustCompile(`^[a-z0-9-]{3,40}$`).MatchString(m.Name) || len(m.Summary) > 160 || len(m.Summary) == 0 {
		t.Fatal("invalid manifest", m)
	}
	if m.TokenRole != "bookkeeper" || m.API != 2 || m.License != "MIT" || m.Pricing["model"] != "free" || m.Status != "preview" {
		t.Fatal(m)
	}
	files, err := filepath.Glob("../../testdata/openapi-*.json")
	if err != nil || len(files) == 0 {
		t.Fatal(files, err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		cat, err := catalog.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[ManifestOperation]bool{}
		for _, op := range m.Operations {
			if seen[op] {
				t.Fatal("duplicate", op)
			}
			seen[op] = true
			match := false
			for _, candidate := range cat.Operations {
				if candidate.Method == op.Method && candidate.Path == op.Path {
					match = true
				}
			}
			if !match {
				t.Fatal(file, "missing operation", op)
			}
			allowed := false
			for _, prefix := range m.Prefixes {
				if strings.HasPrefix(op.Path, prefix) {
					allowed = true
				}
			}
			if !allowed {
				t.Fatal("prefix missing", op)
			}
		}
		for _, row := range append(Reads(), Writes()...) {
			if !row.Catalog && row.Name != "reports run" && !seen[ManifestOperation{row.Method, row.Path}] {
				t.Fatal("missing named operation", row.Name)
			}
		}
	}
	if len(m.Contributes.Service.Downloads) != 6 || m.Contributes.Skills == nil || len(m.Contributes.Skills) != 0 {
		t.Fatal(m.Contributes)
	}
	sums := ""
	for target, download := range m.Contributes.Service.Downloads {
		if !strings.Contains(download.URL, "/releases/download/v1.2.3/") || download.SHA256 != strings.Repeat("0", 64) {
			t.Fatal(download)
		}
		if strings.HasPrefix(target, "windows-") != strings.HasSuffix(download.URL, ".zip") {
			t.Fatal(target, download)
		}
		name := download.URL[strings.LastIndex(download.URL, "/")+1:]
		sums += strings.Repeat("a", 64) + "  " + name + "\n"
	}
	path := filepath.Join(t.TempDir(), "SHA256SUMS")
	if err = os.WriteFile(path, []byte(sums), 0600); err != nil {
		t.Fatal(err)
	}
	checked, err := Manifest("v1.2.3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, download := range checked.Contributes.Service.Downloads {
		if download.SHA256 != strings.Repeat("a", 64) {
			t.Fatal(download)
		}
	}
	for _, bad := range []string{"", "not-a-hash  foo\n", strings.Repeat("a", 64) + "  foo\n", sums + sums} {
		if err = os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = Manifest("v1.2.3", path); err == nil {
			t.Fatal("accepted bad checksums", bad)
		}
	}
}

func TestManifestVersions(t *testing.T) {
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
	for _, version := range []string{"v1.2.3", "1.2.3", "0.0.0"} {
		if _, err := Manifest(version, ""); err != nil {
			t.Fatal(version, err)
		}
	}
	for _, version := range []string{"dev", "", "v0.1.0-rc.1", "1.2.3+build.42", "test", "v1.2", "1.02.3", "1.2.3-01", "1.2.3/invalid"} {
		if _, err := Manifest(version, ""); err == nil {
			t.Fatal("accepted invalid semver", version)
		}
	}
}

func TestManifestPrefixesRejectShortPaths(t *testing.T) {
	for _, path := range []string{"", "/", "/api", "/api/", "api/invoices", "/other/invoices"} {
		if _, err := operationPrefixes([]ManifestOperation{{Method: "GET", Path: path}}); err == nil {
			t.Fatal("accepted malformed path", path)
		}
	}
	got, err := operationPrefixes([]ManifestOperation{{Path: "/api/invoices/{invoice_id}"}, {Path: "/api/customers"}, {Path: "/api/invoices"}})
	if err != nil || strings.Join(got, ",") != "/api/customers,/api/invoices" {
		t.Fatal(got, err)
	}
}
