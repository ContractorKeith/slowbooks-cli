package catalog

import "strings"

// QueryEnum prefers the query schema, then the list response's field schema.
// 2.31.10 declares invoice status on its response rather than its query.
func (c *Catalog) QueryEnum(op Operation, name string) []string {
	for _, p := range op.Params {
		if p.In == "query" && p.Name == name {
			if values := c.enum(p.Schema, 0); len(values) > 0 {
				return values
			}
		}
	}
	paths, _ := c.Document["paths"].(map[string]any)
	path, _ := paths[op.Path].(map[string]any)
	operation, _ := path[strings.ToLower(op.Method)].(map[string]any)
	responses, _ := operation["responses"].(map[string]any)
	response, _ := c.reference(responses["200"]).(map[string]any)
	content, _ := response["content"].(map[string]any)
	media, _ := content["application/json"].(map[string]any)
	schema, _ := c.reference(media["schema"]).(map[string]any)
	item, _ := c.reference(schema["items"]).(map[string]any)
	props, _ := item["properties"].(map[string]any)
	return c.enum(props[name], 0)
}
func (c *Catalog) enum(schema any, depth int) []string {
	if depth > 8 {
		return nil
	}
	obj, _ := c.reference(schema).(map[string]any)
	var values []string
	if items, ok := obj["enum"].([]any); ok {
		for _, item := range items {
			if s, ok := item.(string); ok {
				values = append(values, s)
			}
		}
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if items, ok := obj[key].([]any); ok {
			for _, item := range items {
				values = append(values, c.enum(item, depth+1)...)
			}
		}
	}
	return values
}
