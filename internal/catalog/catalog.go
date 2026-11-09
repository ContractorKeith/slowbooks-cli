// Package catalog models the server's disposable operation catalog.
package catalog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Param struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
	Schema   any    `json:"schema,omitempty"`
}
type Operation struct {
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	Summary   string   `json:"summary"`
	Tags      []string `json:"tags"`
	Params    []Param  `json:"params"`
	Body      any      `json:"body_schema"`
	Keyed     bool     `json:"keyed"`
	FileRoute bool     `json:"file_route"`
	RoleNotes string   `json:"role_notes,omitempty"`
}
type Catalog struct {
	Operations    []Operation
	Document      map[string]any
	Version, Path string
	Warning       string
}

// Parse accepts a structurally valid OpenAPI document, including inherited parameters.
func Parse(data []byte) (*Catalog, error) {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok || doc["openapi"] == nil {
		return nil, fmt.Errorf("invalid OpenAPI catalog")
	}
	c := &Catalog{Document: doc}
	for path, raw := range paths {
		if !strings.HasPrefix(path, "/api/") && path != "/openapi.json" {
			continue
		}
		item, _ := raw.(map[string]any)
		for _, method := range []string{"get", "head", "post", "put", "patch", "delete"} {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			o := Operation{Method: strings.ToUpper(method), Path: path, Summary: str(op["summary"]), Tags: []string{}, Params: []Param{}, RoleNotes: str(op["description"])}
			if tags, ok := op["tags"].([]any); ok {
				for _, tag := range tags {
					o.Tags = append(o.Tags, str(tag))
				}
			}
			for _, source := range []any{item["parameters"], op["parameters"]} {
				params, _ := source.([]any)
				for _, raw := range params {
					p, _ := c.reference(raw).(map[string]any)
					if p == nil {
						continue
					}
					param := Param{str(p["name"]), str(p["in"]), p["required"] == true, p["schema"]}
					replaced := false
					for i, old := range o.Params {
						if old.Name == param.Name && old.In == param.In {
							o.Params[i] = param
							replaced = true
						}
					}
					if !replaced {
						o.Params = append(o.Params, param)
					}
				}
			}
			rb, _ := c.reference(op["requestBody"]).(map[string]any)
			content, _ := rb["content"].(map[string]any)
			media, _ := content["application/json"].(map[string]any)
			o.Body = media["schema"]
			o.Keyed = IsKeyed(o.Method, path)
			for _, p := range o.Params {
				if p.In == "header" && strings.EqualFold(p.Name, "Idempotency-Key") {
					o.Keyed = true
				}
			}
			o.FileRoute = fileRoute(o.Method, path, op, c)
			c.Operations = append(c.Operations, o)
		}
	}
	sort.Slice(c.Operations, func(i, j int) bool {
		a, b := c.Operations[i], c.Operations[j]
		if a.Path == b.Path {
			return a.Method < b.Method
		}
		return a.Path < b.Path
	})
	return c, nil
}
func str(v any) string { s, _ := v.(string); return s }

// IsKeyed recognizes payment routes even when the catalog is incomplete.
func IsKeyed(method, path string) bool {
	if !strings.EqualFold(method, "POST") {
		return false
	}
	parts := strings.Split(path, "/")
	if len(parts) != 3 && len(parts) != 5 {
		return false
	}
	if parts[0] != "" || parts[1] != "api" || (parts[2] != "payments" && parts[2] != "bill-payments") {
		return false
	}
	return len(parts) == 3 || parts[3] != "" && parts[4] == "discount"
}

func IsFile(path string) bool {
	for _, part := range strings.Split(strings.ToLower(path), "/") {
		switch part {
		case "pdf", "csv", "iif", "print", "print-preview", "download":
			return true
		}
		if strings.HasSuffix(part, ".pdf") || strings.HasSuffix(part, ".csv") {
			return true
		}
	}
	return false
}

// Lookup accepts either a template or its concrete path, keeping keyed guards intact.
func (c *Catalog) Lookup(method, path string) (Operation, bool) {
	for _, o := range c.Operations {
		if !strings.EqualFold(method, o.Method) {
			continue
		}
		a, b := strings.Split(path, "/"), strings.Split(o.Path, "/")
		if len(a) != len(b) {
			continue
		}
		match := true
		for i := range a {
			if a[i] != b[i] && !(strings.HasPrefix(b[i], "{") && strings.HasSuffix(b[i], "}") && a[i] != "") {
				match = false
			}
		}
		if match {
			return o, true
		}
	}
	return Operation{}, false
}
