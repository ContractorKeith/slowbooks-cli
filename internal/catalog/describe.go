package catalog

import (
	"sort"
	"strings"
)

func (c *Catalog) reference(v any) any {
	obj, ok := v.(map[string]any)
	if !ok {
		return v
	}
	ref, ok := obj["$ref"].(string)
	if !ok || !strings.HasPrefix(ref, "#/") {
		return v
	}
	var found any = c.Document
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		m, ok := found.(map[string]any)
		if !ok {
			return v
		}
		found = m[strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")]
	}
	if found == nil {
		return v
	}
	return found
}

// Describe resolves local schemas within fixed depth and node budgets, including cycles.
func (c *Catalog) Describe(o Operation) any {
	budget := 600
	cut := false
	chars := 16000
	var expand func(any, int) any
	expand = func(v any, depth int) any {
		budget--
		if text, ok := v.(string); ok {
			chars -= len(text)
		}
		if budget < 0 || depth > 8 || chars < 0 {
			cut = true
			return "<truncated>"
		}
		v = c.reference(v)
		switch x := v.(type) {
		case map[string]any:
			out := map[string]any{}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if budget <= 0 {
					cut = true
					out["truncated"] = true
					break
				}
				chars -= len(k)
				if chars < 0 {
					cut = true
					out["truncated"] = true
					break
				}
				out[k] = expand(x[k], depth+1)
			}
			return out
		case []any:
			out := []any{}
			for _, value := range x {
				if budget <= 0 {
					cut = true
					break
				}
				out = append(out, expand(value, depth+1))
			}
			return out
		case string:
			if len(x) > 2000 {
				cut = true
				return x[:2000] + "<truncated>"
			}
		}
		return v
	}
	o.Body = expand(o.Body, 0)
	o.Params = append([]Param{}, o.Params...)
	for i := range o.Params {
		o.Params[i].Schema = expand(o.Params[i].Schema, 0)
	}
	if len(o.RoleNotes) > 2000 {
		o.RoleNotes = o.RoleNotes[:2000] + "<truncated>"
		cut = true
	}
	return struct {
		Operation
		Truncated bool `json:"truncated"`
	}{o, cut}
}

// Find ranks path matches above summary and tag matches, with deterministic ties.
func (c *Catalog) Find(words []string, limit int) []Operation {
	type hit struct {
		op             Operation
		score, matched int
	}
	hits := []hit{}
	for _, o := range c.Operations {
		score, matched := 0, 0
		for _, word := range words {
			word = strings.ToLower(word)
			value := 0
			if pathMatches(o.Path, word) {
				value = 4
			} else if strings.Contains(strings.ToLower(o.Summary), word) {
				value = 2
			} else if strings.Contains(strings.ToLower(strings.Join(o.Tags, " ")), word) {
				value = 1
			}
			score += value
			if value > 0 {
				matched++
			}
		}
		if score > 0 {
			hits = append(hits, hit{o, score, matched})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].matched != hits[j].matched {
			return hits[i].matched > hits[j].matched
		}
		return hits[i].score > hits[j].score
	})
	out := []Operation{}
	for i, h := range hits {
		if i >= limit {
			break
		}
		h.op.Body = nil
		h.op.RoleNotes = ""
		out = append(out, h.op)
	}
	return out
}

func pathMatches(path, word string) bool {
	for _, part := range strings.FieldsFunc(strings.ToLower(path), func(r rune) bool { return r == '/' || r == '-' || r == '_' || r == '{' || r == '}' }) {
		if strings.HasPrefix(part, word) {
			return true
		}
	}
	return false
}
