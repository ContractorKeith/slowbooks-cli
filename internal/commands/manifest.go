package commands

import (
	"bufio"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

//go:embed report_paths.json
var reportPaths []byte

type ManifestOperation struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
type Download struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}
type Service struct {
	Downloads map[string]Download `json:"downloads"`
	Run       string              `json:"run"`
}
type PluginManifest struct {
	Schema      int                 `json:"schema"`
	Name        string              `json:"name"`
	DisplayName string              `json:"display_name"`
	Version     string              `json:"version"`
	Summary     string              `json:"summary"`
	Author      map[string]string   `json:"author"`
	License     string              `json:"license"`
	Homepage    string              `json:"homepage"`
	Repository  string              `json:"repository"`
	API         int                 `json:"sbp_api_major"`
	TokenRole   string              `json:"token_role"`
	Prefixes    []string            `json:"prefixes"`
	Operations  []ManifestOperation `json:"operations"`
	Contributes struct {
		Skills  []string `json:"skills"`
		Service Service  `json:"service"`
		Links   []string `json:"links"`
	} `json:"contributes"`
	Pricing map[string]string `json:"pricing"`
	Status  string            `json:"status"`
}

// NamedOperations expands the report placeholder into verified concrete paths.
func NamedOperations() []ManifestOperation {
	var reports []string
	if err := json.Unmarshal(reportPaths, &reports); err != nil {
		panic(err)
	}
	result := []ManifestOperation{}
	for _, row := range append(Reads(), Writes()...) {
		if row.Catalog {
			continue
		}
		if row.Name == "reports run" {
			for _, path := range reports {
				result = append(result, ManifestOperation{row.Method, path})
			}
		} else {
			result = append(result, ManifestOperation{row.Method, row.Path})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Method < result[j].Method
		}
		return result[i].Path < result[j].Path
	})
	return result
}

var marketplaceVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Manifest builds the directory contract without contacting a server.
// The marketplace accepts stable x.y.z versions only.
func Manifest(version, checksumFile string) (PluginManifest, error) {
	version = strings.TrimPrefix(version, "v")
	if !marketplaceVersion.MatchString(version) {
		return PluginManifest{}, fmt.Errorf("marketplace manifest requires a stable x.y.z build version: %s", version)
	}
	sums, err := readChecksums(checksumFile)
	if err != nil {
		return PluginManifest{}, err
	}
	repo := "https://github.com/ContractorKeith/slowbooks-cli"
	m := PluginManifest{Schema: 1, Name: "sbp-cli", DisplayName: "sbp: the command line", Version: version,
		Summary: "SlowBooks Pro from the shell: read, write, report and download with JSON output and a dry run on every write.",
		Author:  map[string]string{"name": "ContractorKeith", "url": "https://github.com/ContractorKeith"}, License: "MIT",
		Homepage: "https://plugins.kodade.com/plugins/sbp-cli/", Repository: repo, API: 2, TokenRole: "bookkeeper",
		Operations: NamedOperations(), Pricing: map[string]string{"model": "free"}, Status: "preview"}
	m.Prefixes, err = operationPrefixes(m.Operations)
	if err != nil {
		return PluginManifest{}, err
	}
	m.Contributes.Skills, m.Contributes.Links = []string{}, []string{}
	m.Contributes.Service = Service{Downloads: map[string]Download{}, Run: "sbp auth login --url <server>; sbp invoices list --json"}
	for _, target := range []string{"linux-x64", "linux-arm64", "macos-x64", "macos-arm64", "windows-x64", "windows-arm64"} {
		ext := ".tar.gz"
		if strings.HasPrefix(target, "windows-") {
			ext = ".zip"
		}
		name := "sbp_" + version + "_" + target + ext
		sum := strings.Repeat("0", 64)
		if checksumFile != "" {
			var ok bool
			sum, ok = sums[name]
			if !ok {
				return PluginManifest{}, fmt.Errorf("missing checksum for %s", name)
			}
		}
		m.Contributes.Service.Downloads[target] = Download{repo + "/releases/download/v" + version + "/" + name, sum}
	}
	return m, nil
}

func readChecksums(path string) (map[string]string, error) {
	result := map[string]string{}
	if path == "" {
		return result, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksum line")
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("invalid SHA256 checksum")
		}
		name := strings.TrimPrefix(fields[1], "*")
		if _, ok := result[name]; ok {
			return nil, fmt.Errorf("duplicate checksum for %s", name)
		}
		result[name] = strings.ToLower(fields[0])
	}
	return result, scanner.Err()
}

// operationPrefixes rejects malformed operation paths before deriving scopes.
func operationPrefixes(operations []ManifestOperation) ([]string, error) {
	seen := map[string]bool{}
	for _, op := range operations {
		parts := strings.Split(op.Path, "/")
		if len(parts) < 3 || parts[0] != "" || parts[1] != "api" || parts[2] == "" {
			return nil, fmt.Errorf("cannot derive prefix from operation path %q", op.Path)
		}
		seen[strings.Join(parts[:3], "/")] = true
	}
	prefixes := make([]string, 0, len(seen))
	for prefix := range seen {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	return prefixes, nil
}
