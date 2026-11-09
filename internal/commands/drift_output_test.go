package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
)

func TestDriftColumnsBrokenCopy(t *testing.T) {
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
	raw, err := os.ReadFile("../../testdata/openapi-2.31.10.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	delete(schemas["CustomerResponse"].(map[string]any)["properties"].(map[string]any), "email")
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "columns.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = checkDrift(path); err == nil || !strings.Contains(err.Error(), "column email") {
		t.Fatalf("missing column must fail drift: %v", err)
	}
}

func TestDriftReportMappings(t *testing.T) {
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
	raw, err := os.ReadFile("../../testdata/openapi-2.31.10.json")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Required report mappings must fail drift if the snapshot stops declaring them.
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["paths"].(map[string]any)["/api/reports/profit-loss"].(map[string]any)["get"].(map[string]any)["parameters"] = []any{}
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = checkDrift(path); err == nil || !strings.Contains(err.Error(), "report date") {
		t.Fatalf("missing report dates must fail drift: %v", err)
	}
	// Check the actual production mapper for every discovered report, not just P&L.
	for _, report := range Reports(cat) {
		op, _ := cat.Lookup("GET", "/api/reports/"+report.Name)
		for _, flag := range []string{"from", "to"} {
			target := ReportParam(op, flag)
			if target == "" {
				continue
			}
			found := false
			for _, p := range op.Params {
				if p.In == "query" && p.Name == target {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s --%s maps to undeclared %s", report.Name, flag, target)
			}
		}
	}
}
