package fakeserver

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWriteFixtureFailureReturns500(t *testing.T) {
	for _, fixture := range []string{"", "[]", "null", "not-json"} {
		t.Run(fixture, func(t *testing.T) {
			defer func() {
				if v := recover(); v != nil {
					t.Errorf("fixture failure panicked: %v", v)
				}
			}()
			fake := New()
			fake.fixtures = fstest.MapFS{}
			if fixture != "" {
				fake.fixtures = fstest.MapFS{"fake/invoices.json": &fstest.MapFile{Data: []byte(fixture)}}
			}
			response := httptest.NewRecorder()
			fake.ServeHTTP(response, httptest.NewRequest("POST", "/api/invoices", strings.NewReader(`{}`)))
			if response.Code != 500 || !strings.Contains(response.Body.String(), "Write fixture unavailable") {
				t.Fatal(response.Code, response.Body.String())
			}
		})
	}
}
