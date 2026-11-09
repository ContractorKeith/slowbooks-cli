package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
)

// checkDrift requires exact template and argument names, rather than fuzzy lookup.
func checkDrift(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cat, err := catalog.Parse(raw)
	if err != nil {
		return err
	}
	for _, row := range Reads() {
		if row.Catalog {
			continue
		}
		if row.Name == "reports run" {
			if len(Reports(cat)) == 0 {
				return fmt.Errorf("no JSON reports")
			}
			found := false
			for _, r := range Reports(cat) {
				if r.Name == "profit-loss" {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("missing profit-loss report")
			}
			if err := checkReportMappings(cat); err != nil {
				return err
			}
			continue
		}
		var match *catalog.Operation
		for _, op := range cat.Operations {
			if op.Method == row.Method && op.Path == row.Path {
				copy := op
				match = &copy
				break
			}
		}
		if match == nil {
			return fmt.Errorf("%s: missing %s %s", row.Name, row.Method, row.Path)
		}
		if err := checkColumns(cat, row); err != nil {
			return err
		}
		for _, arg := range row.Args {
			location := "query"
			if strings.Contains(row.Path, "{"+arg+"}") {
				location = "path"
			}
			found := false
			for _, p := range match.Params {
				if p.Name == arg && p.In == location {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("%s: missing %s argument %s", row.Name, location, arg)
			}
		}
		for _, flag := range row.Flags {
			found := false
			for _, p := range match.Params {
				if p.In == "query" && p.Name == flag.Param {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("%s: missing flag parameter %s", row.Name, flag.Param)
			}
		}
	}
	return nil
}
func TestDrift(t *testing.T) {
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
	files, err := filepath.Glob("../../testdata/openapi-*.json")
	if err != nil || len(files) == 0 {
		t.Fatal(files, err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			if err := checkDrift(file); err != nil {
				t.Fatal(err)
			}
			t.Run("broken-copy", func(t *testing.T) {
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var doc map[string]any
				if err = json.Unmarshal(raw, &doc); err != nil {
					t.Fatal(err)
				}
				delete(doc["paths"].(map[string]any), "/api/customers/{customer_id}")
				raw, err = json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				broken := filepath.Join(t.TempDir(), "broken.json")
				if err = os.WriteFile(broken, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err = checkDrift(broken); err == nil || !strings.Contains(err.Error(), "customers show") {
					t.Fatalf("expected missing customer show: %v", err)
				}
			})
		})
	}
}
