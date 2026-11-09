package commands

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
)

// sharedReports uses Reports' catalog/FileRoute policy and keeps compatibility
// with every pinned snapshot, allowing newer snapshots to add reports.
func sharedReports(catalogs []*catalog.Catalog) []string {
	counts := map[string]int{}
	for _, cat := range catalogs {
		for _, report := range Reports(cat) {
			counts["/api/reports/"+report.Name]++
		}
	}
	paths := []string{}
	for path, count := range counts {
		if count == len(catalogs) {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	return paths
}

func TestReportsSecondSnapshotAndFileRoutes(t *testing.T) {
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
	raw := []byte(`{"openapi":"3.1.0","paths":{
 "/api/reports/profit-loss":{"get":{"responses":{"200":{"content":{"application/json":{}}}}}},
 "/api/reports/file":{"get":{"responses":{"200":{"content":{"application/pdf":{}}}}}},
 "/api/reports/pdf":{"get":{"responses":{"200":{"content":{"application/json":{}}}}}},
 "/api/reports/undeclared":{"get":{}},
 "/api/reports/mixed":{"get":{"responses":{"200":{"content":{"application/pdf":{}}},"201":{"content":{"application/json":{}}}}}}
 }}`)
	first, err := catalog.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	paths["/api/reports/new-in-2.32"] = map[string]any{"get": map[string]any{}}
	secondRaw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	second, err := catalog.Parse(secondRaw)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/api/reports/mixed", "/api/reports/profit-loss", "/api/reports/undeclared"}
	if got := sharedReports([]*catalog.Catalog{first, second}); !slices.Equal(got, want) {
		t.Fatal(got, want)
	}
	// The new report is reachable on that server, without breaking older versions.
	if len(Reports(second)) != len(Reports(first))+1 {
		t.Fatal("new report lost")
	}
}
