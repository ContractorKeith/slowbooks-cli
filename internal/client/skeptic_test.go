package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestSkepticDetailMessages(t *testing.T) {
	isolate(t)
	for _, detail := range []string{`{"message":"duplicate customer","extra":1}`, `[{"msg":"duplicate customer"}]`, `[{"message":"duplicate customer","msg":"other"}]`} {
		e := responseError(409, []byte(`{"detail":`+detail+`}`), false)
		raw, _ := json.Marshal(e.Detail)
		if e.Message != "duplicate customer" || string(raw) != detail {
			t.Fatalf("%+v detail=%s", e, raw)
		}
	}
}
func TestSkepticKeyRules(t *testing.T) {
	isolate(t)
	for _, key := range []string{"chk 1042", "a/b", "é", strings.Repeat("a", 129), "a\nb"} {
		if output.ExitCode(ValidateKey(key)) != 2 {
			t.Errorf("accepted %q", key)
		}
	}
	if err := ValidateKey(strings.Repeat("a", 128)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateKey("ABCabc012._:-"); err != nil {
		t.Fatal(err)
	}
}
func TestSkepticCompanyHeader(t *testing.T) {
	isolate(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; w.WriteHeader(204) }))
	defer srv.Close()
	for _, company := range []string{"a\nb", "a\rb", "a\x00b", "a\x7fb"} {
		c, err := New(context.Background(), Options{URL: srv.URL, Company: company})
		if err == nil {
			_, err = c.Preview("POST", "/api/customers", nil, "")
		}
		if output.ExitCode(err) != 2 {
			t.Errorf("company %q: %v", company, err)
		}
	}
	if hits != 0 {
		t.Fatal(hits)
	}
}
func TestSkepticWriteGatewayAdvice(t *testing.T) {
	isolate(t)
	for _, status := range []int{502, 503, 504} {
		hits := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; w.WriteHeader(status) }))
		c, err := New(context.Background(), Options{URL: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"", "check-1"} {
			_, err = c.ForOperation(true).Do(context.Background(), "POST", "/api/payments", nil, key)
			e := output.AsError(err)
			if output.ExitCode(err) != 8 || e.Hint == "" || e.Retryable != (key != "") || !WriteReachedWire(err) {
				t.Errorf("status %d key %q: %+v", status, key, e)
			}
		}
		srv.Close()
		if hits != 2 {
			t.Fatal(hits)
		}
	}
}
func TestSkepticDataEncodings(t *testing.T) {
	isolate(t)
	b, err := ReadData(strings.NewReader("\xef\xbb\xbf{}"), "-")
	if err != nil || string(b) != "{}" {
		t.Errorf("BOM: %q %v", b, err)
	}
	for _, data := range []string{"\xff\xfe{\x00}", "\xfe\xff\x00{\x00}"} {
		_, err = ReadData(strings.NewReader(data), "-")
		if output.ExitCode(err) != 2 || !strings.Contains(err.Error(), "UTF-16 is not supported; save as UTF-8") {
			t.Errorf("UTF16: %v", err)
		}
	}
}
