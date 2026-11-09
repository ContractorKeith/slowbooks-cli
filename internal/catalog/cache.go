package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

var safeComponent = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func cacheDir(profile string) (string, error) {
	if profile == "" {
		profile = "env"
	}
	if !safeComponent.MatchString(profile) {
		return "", fmt.Errorf("invalid catalog profile")
	}
	root := os.Getenv("SLOWBOOKS_CACHE_DIR")
	if root == "" {
		var err error
		root, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(root, "sbp")
	}
	return filepath.Join(root, profile), nil
}

// Load checks the current version unless offline; dry-runs never contact the server.
func Load(ctx context.Context, cl *client.Client, profile string, offline bool) (*Catalog, error) {
	dir, err := cacheDir(profile)
	if err != nil {
		return nil, err
	}
	if offline {
		version, err := os.ReadFile(filepath.Join(dir, "current"))
		if err == nil && safeComponent.Match(version) {
			path := filepath.Join(dir, "openapi-"+string(version)+".json")
			data, err := os.ReadFile(path)
			if err == nil {
				cat, err := Parse(data)
				if err == nil {
					cat.Path = path
					cat.Version = string(version)
					return cat, nil
				}
			}
		}
		return nil, &output.Error{Code: "usage", Message: "dry-run needs a cached catalog", Hint: "run sbp ops find <words> first to cache the catalog"}
	}

	data, err := cl.Do(ctx, "GET", "/api/system", nil, "")
	if err != nil {
		return nil, err
	}
	var system struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &system) != nil || !safeComponent.MatchString(system.Version) {
		return nil, &output.Error{Code: "server", Message: "invalid server version"}
	}
	path := filepath.Join(dir, "openapi-"+system.Version+".json")
	if data, err = os.ReadFile(path); err == nil {
		if cat, e := Parse(data); e == nil {
			cat.Path = path
			cat.Version = system.Version
			return cat, writeCache(filepath.Join(dir, "current"), []byte(cat.Version))
		}
	}
	data, err = cl.Do(ctx, "GET", "/openapi.json", nil, "")
	if err != nil {
		return nil, err
	}
	cat, err := Parse(data)
	if err != nil {
		return nil, &output.Error{Code: "server", Message: "invalid OpenAPI catalog"}
	}
	if err = writeCache(path, data); err != nil {
		return nil, err
	}
	cat.Path = path
	cat.Version = system.Version
	return cat, writeCache(filepath.Join(dir, "current"), []byte(cat.Version))
}
func writeCache(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".openapi-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
