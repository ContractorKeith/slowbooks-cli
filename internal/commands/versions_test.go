package commands

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestVerifiedVersionsMatchSnapshots(t *testing.T) {
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "openapi-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshots := make([]string, 0, len(files))
	for _, file := range files {
		snapshots = append(snapshots, strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "openapi-"), ".json"))
	}
	verified := VerifiedVersions()
	slices.Sort(verified)
	slices.Sort(snapshots)
	if !slices.Equal(verified, snapshots) {
		t.Fatalf("verified versions %v do not match snapshots %v", verified, snapshots)
	}
}
