package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

var safeComponent = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func cacheDir(profile string) (string, error) {
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
		root = filepath.Join(root, "slowbooks")
	}
	return filepath.Join(root, profile), nil
}

// Version filenames preserve ordinary releases; unusual versions get a bounded,
// readable prefix and hash so sanitizing cannot collide or escape the cache.
func versionComponent(version string) string {
	if safeComponent.MatchString(version) && len(version) <= 100 {
		return version
	}
	prefix := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, version)
	if len(prefix) > 80 {
		prefix = prefix[:80]
	}
	if prefix == "" {
		prefix = "unknown"
	}
	sum := sha256.Sum256([]byte(version))
	return "v-" + prefix + "-" + hex.EncodeToString(sum[:8])
}

// Load checks the server version unless offline. Cache writes are best effort;
// callers render Warning through output while still using the fetched catalog.
func Load(ctx context.Context, cl *client.Client, profile string, offline bool) (*Catalog, error) {
	if profile == "" {
		profile = "env-" + cl.CacheIdentity()
	}
	dir, dirErr := cacheDir(profile)
	if offline {
		if dirErr != nil {
			return nil, dirErr
		}
		version, err := os.ReadFile(filepath.Join(dir, "current"))
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			return nil, &output.Error{Code: "usage", Message: "dry-run needs a cached catalog", Hint: "run slowbooks ops find <words> first to cache the catalog"}
		}
		path := filepath.Join(dir, "openapi-"+versionComponent(string(version))+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		cat, err := Parse(data)
		if err != nil {
			return nil, err
		}
		cat.Path = path
		cat.Version = string(version)
		return cat, nil
	}
	data, err := cl.Do(ctx, "GET", "/api/system", nil, "")
	if err != nil {
		return nil, err
	}
	var system struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &system) != nil {
		return nil, &output.Error{Code: "error", Message: "invalid server version response"}
	}
	path := ""
	var cat *Catalog
	if dirErr == nil {
		path = filepath.Join(dir, "openapi-"+versionComponent(system.Version)+".json")
		if data, err = os.ReadFile(path); err == nil {
			cat, _ = Parse(data)
		}
	}
	fetched := cat == nil
	if fetched {
		data, err = cl.Do(ctx, "GET", "/openapi.json", nil, "")
		if err != nil {
			return nil, err
		}
		cat, err = Parse(data)
		if err != nil {
			return nil, &output.Error{Code: "error", Message: "invalid OpenAPI catalog"}
		}
	}
	cat.Path = path
	cat.Version = system.Version
	cacheErr := dirErr
	if cacheErr == nil && fetched {
		cacheErr = writeCache(path, data)
	}
	if cacheErr == nil {
		cacheErr = writeCache(filepath.Join(dir, "current"), []byte(system.Version))
	}
	if cacheErr != nil {
		cat.Warning = "could not write catalog cache: " + cacheErr.Error()
	}
	return cat, nil
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
