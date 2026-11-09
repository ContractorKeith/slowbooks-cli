package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReviewChainedReferences(t *testing.T) {
	isolate(t)
	c, err := Parse([]byte(`{"openapi":"3.1.0","paths":{"/api/test":{"post":{"parameters":[{"$ref":"#/components/parameters/A"}],"requestBody":{"$ref":"#/components/requestBodies/A"}}}},"components":{"parameters":{"A":{"$ref":"#/components/parameters/B"},"B":{"in":"header","name":"Idempotency-Key"}},"requestBodies":{"A":{"$ref":"#/components/requestBodies/B"},"B":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/A"}}}}},"schemas":{"A":{"$ref":"#/components/schemas/B"},"B":{"type":"object","properties":{"amount":{"type":"number"}}},"cycle":{"$ref":"#/components/schemas/cycle"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	op, _ := c.Lookup("POST", "/api/test")
	if !op.Keyed {
		t.Error("chained header lost")
	}
	data, _ := json.Marshal(c.Describe(op))
	if !strings.Contains(string(data), "amount") || strings.Contains(string(data), "$ref") {
		t.Error(string(data))
	}
	// Alias cycles must terminate and report a bounded cut.
	op.Body = map[string]any{"$ref": "#/components/schemas/cycle"}
	data, _ = json.Marshal(c.Describe(op))
	if !strings.Contains(string(data), `"truncated":true`) {
		t.Error(string(data))
	}
}
func TestReviewFileRouteMethodsAndMedia(t *testing.T) {
	isolate(t)
	data := snapshot(t)
	c, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range c.Operations {
		if op.Method != "GET" && op.Method != "HEAD" && op.FileRoute {
			t.Errorf("write marked file: %s %s", op.Method, op.Path)
		}
	}
	op, ok := c.Lookup("GET", "/api/payroll/{run_id}/nacha")
	if !ok || !op.FileRoute {
		t.Errorf("text route missed: %+v", op)
	}
	c, err = Parse([]byte(`{"openapi":"3.1.0","paths":{"/api/text":{"get":{"responses":{"200":{"content":{"text/plain":{}}}}}},"/api/mixed":{"get":{"responses":{"200":{"content":{"text/plain":{}}},"201":{"content":{"application/json":{}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	text, _ := c.Lookup("GET", "/api/text")
	mixed, _ := c.Lookup("GET", "/api/mixed")
	if !text.FileRoute || mixed.FileRoute {
		t.Fatal(text, mixed)
	}
}
func TestReviewExactResourceRanking(t *testing.T) {
	isolate(t)
	c, err := Parse(snapshot(t))
	if err != nil {
		t.Fatal(err)
	}
	found := c.Find([]string{"customer"}, 20)
	if len(found) == 0 || found[0].Path != "/api/customers" {
		t.Fatal(found)
	}
}
