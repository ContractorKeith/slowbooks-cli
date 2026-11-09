package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
)

func TestSkepticKeyedShape(t *testing.T) {
	isolate(t)
	for _, path := range []string{"/api/payments/5/discount", "/api/payments/{id}/discount", "/api/bill-payments/{renamed}/discount"} {
		if !IsKeyed("POST", path) {
			t.Error(path)
		}
		if IsKeyed("GET", path) {
			t.Error(path)
		}
	}
	for _, path := range []string{"/api/payments//discount", "/api/payments/5/other", "/api/payments/5/discount/extra"} {
		if IsKeyed("POST", path) {
			t.Error(path)
		}
	}
}
func TestSkepticEnvCacheIsolation(t *testing.T) {
	isolate(t)
	var dirs []string
	for range 2 {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/system" {
				_, _ = w.Write([]byte(`{"version":"2.31.10"}`))
				return
			}
			_, _ = w.Write(snapshot(t))
		}))
		defer srv.Close()
		cl, err := client.New(context.Background(), client.Options{URL: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		cat, err := Load(context.Background(), cl, "", false)
		if err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, filepath.Dir(cat.Path))
	}
	if dirs[0] == dirs[1] {
		t.Fatal(dirs)
	}
}

func TestSkepticDeclaredEnumPrecedence(t *testing.T) {
	isolate(t)
	cat, err := Parse([]byte(`{"openapi":"3.1.0","paths":{"/api/bills":{"get":{"parameters":[{"in":"query","name":"status","schema":{"anyOf":[{"$ref":"#/components/schemas/BillStatus"},{"type":"null"}]}}]}}},"components":{"schemas":{"BillStatus":{"enum":["new-status","paid"]}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	op, _ := cat.Lookup("GET", "/api/bills")
	values := cat.QueryEnum(op, "status")
	if len(values) != 2 || values[0] != "new-status" {
		t.Fatal(values)
	}
}

func TestKeyedRoutesIgnorePathCase(t *testing.T) {
	isolate(t)
	for _, path := range []string{"/API/PAYMENTS", "/api/BILL-PAYMENTS", "/api/Payments/5/Discount", "/api/BILL-PAYMENTS/{ID}/DISCOUNT"} {
		if !IsKeyed("POST", path) {
			t.Error(path)
		}
	}
}
func TestCacheDirectoryRequiresExplicitIdentity(t *testing.T) {
	isolate(t)
	if _, err := cacheDir(""); err == nil {
		t.Fatal("empty cache identity accepted")
	}
}
