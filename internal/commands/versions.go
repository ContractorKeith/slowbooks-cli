package commands

import (
	"io/fs"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/testdata"
)

// VerifiedVersions is derived from the embedded pinned snapshots.
func VerifiedVersions() []string {
	files, _ := fs.Glob(testdata.Files, "openapi-*.json")
	versions := make([]string, 0, len(files))
	for _, name := range files {
		versions = append(versions, strings.TrimSuffix(strings.TrimPrefix(name, "openapi-"), ".json"))
	}
	return versions
}
