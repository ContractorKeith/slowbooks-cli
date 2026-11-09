package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/testdata"
)

func isolate(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
}
func snapshot(t *testing.T) []byte {
	t.Helper()
	b, err := testdata.Files.ReadFile("openapi-2.31.10.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestSnapshot(t *testing.T) {
	isolate(t)
	cat, err := Parse(snapshot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Operations) < 100 {
		t.Fatal(len(cat.Operations))
	}
	for _, path := range []string{"/api/payments", "/api/bill-payments", "/api/payments/{payment_id}/discount", "/api/bill-payments/{bill_payment_id}/discount"} {
		op, ok := cat.Lookup("POST", path)
		if !ok || !op.Keyed {
			t.Fatal(path, op)
		}
	}
	op, ok := cat.Lookup("POST", "/api/payments/123/discount")
	if !ok || !op.Keyed {
		t.Fatal(op)
	}
	op, _ = cat.Lookup("POST", "/api/payments")
	b, _ := json.Marshal(cat.Describe(op))
	if strings.Contains(string(b), `"$ref"`) || !strings.Contains(string(b), "properties") {
		t.Fatal(string(b))
	}
	found := false
	for _, o := range cat.Find([]string{"customer"}, 20) {
		if o.Method == "POST" && o.Path == "/api/customers" {
			found = true
		}
	}
	if !found {
		t.Fatal("create customer missing")
	}
	for _, path := range []string{"/api/invoices/1/pdf", "/api/export/csv/foo", "/api/export/iif", "/api/a/print", "/api/a/download", "/api/a/file.pdf"} {
		if !IsFile(path) {
			t.Fatal(path)
		}
	}
}
func TestCacheVersionCorruptionAndOffline(t *testing.T) {
	isolate(t)
	version := "2.31.10"
	fetches, hits := 0, 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/api/system" {
			_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
			return
		}
		fetches++
		_, _ = w.Write(snapshot(t))
	}))
	defer s.Close()
	cl, err := client.New(context.Background(), client.Options{URL: s.URL})
	if err != nil {
		t.Fatal(err)
	}
	load := func(offline bool) *Catalog {
		t.Helper()
		cat, err := Load(context.Background(), cl, "", offline)
		if err != nil {
			t.Fatal(err)
		}
		return cat
	}
	cat := load(false)
	load(false)
	if fetches != 1 {
		t.Fatal(fetches)
	}
	if err = os.WriteFile(cat.Path, []byte(`{"paths":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	load(false)
	if fetches != 2 {
		t.Fatal(fetches)
	}
	version = "2.32.0"
	cat = load(false)
	if fetches != 3 || cat.Version != version {
		t.Fatal(fetches, cat)
	}
	version = "2.31.10"
	load(false)
	before := hits
	offline := load(true)
	if offline.Version != version {
		t.Fatal("offline cache selected the wrong version", offline.Version)
	}
	if hits != before {
		t.Fatal("offline sent HTTP")
	}
	version = "../../escape"
	_, err = Load(context.Background(), cl, "", false)
	if err == nil {
		t.Fatal("unsafe version accepted")
	}
}
func TestInheritedHeadersAndBoundedSchema(t *testing.T) {
	isolate(t)
	cat, err := Parse([]byte(`{"openapi":"3.1.0","paths":{"/api/test":{"parameters":[{"$ref":"#/components/parameters/key"}],"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/loop"}}}}}}},"components":{"parameters":{"key":{"in":"header","name":"Idempotency-Key"}},"schemas":{"loop":{"properties":{"again":{"$ref":"#/components/schemas/loop"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	op, ok := cat.Lookup("POST", "/api/test")
	if !ok || !op.Keyed {
		t.Fatal(op)
	}
	b, _ := json.Marshal(cat.Describe(op))
	if len(b) > 20000 || !strings.Contains(string(b), `"truncated":true`) {
		t.Fatal(string(b))
	}
}

func TestFindRankingAndLimit(t *testing.T) {
	isolate(t)
	c := &Catalog{Operations: []Operation{
		{Path: "/api/a", Summary: "Customer summary"},
		{Path: "/api/b", Tags: []string{"customer"}},
		{Path: "/api/customers"},
	}}
	found := c.Find([]string{"customer"}, 2)
	if len(found) != 2 || found[0].Path != "/api/customers" || found[1].Path != "/api/a" {
		t.Fatal(found)
	}
	if len(c.Find([]string{"no-match"}, 20)) != 0 {
		t.Fatal("unexpected match")
	}
}
