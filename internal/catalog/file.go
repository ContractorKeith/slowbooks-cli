package catalog

import "strings"

// File detection applies only to reads. JSON among any successful media keeps a
// mixed-response operation usable; an absent media declaration is inconclusive.
func fileRoute(method, path string, op map[string]any, c *Catalog) bool {
	if method != "GET" && method != "HEAD" {
		return false
	}
	if IsFile(path) {
		return true
	}
	responses, _ := op["responses"].(map[string]any)
	declared, jsonMedia := false, false
	for status, raw := range responses {
		if !strings.HasPrefix(status, "2") {
			continue
		}
		response, _ := c.reference(raw).(map[string]any)
		content, _ := response["content"].(map[string]any)
		for media := range content {
			declared = true
			if strings.EqualFold(strings.Split(media, ";")[0], "application/json") {
				jsonMedia = true
			}
		}
	}
	return declared && !jsonMedia
}

// responseMedia retains declared successful media for write-response validation.
func responseMedia(op map[string]any, c *Catalog) []string {
	var media []string
	responses, _ := op["responses"].(map[string]any)
	for status, raw := range responses {
		if !strings.HasPrefix(status, "2") {
			continue
		}
		response, _ := c.reference(raw).(map[string]any)
		content, _ := response["content"].(map[string]any)
		for name := range content {
			media = append(media, strings.ToLower(strings.TrimSpace(strings.Split(name, ";")[0])))
		}
	}
	return media
}
