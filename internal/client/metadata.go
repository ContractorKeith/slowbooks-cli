package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// CacheIdentity scopes environment-only discovery to this server and company.
func (c *Client) CacheIdentity() string {
	sum := sha256.Sum256([]byte(c.base.String() + "\x00" + c.options.Company))
	return hex.EncodeToString(sum[:8])
}

// Company is the selected routing header, or empty for the server default.
func (c *Client) Company() string { return c.options.Company }

func validateHeaders(o Options) error {
	for name, value := range map[string]string{"company": o.Company, "Authorization": o.Token, "User-Agent": o.Version} {
		for i := 0; i < len(value); i++ {
			if value[i] < 32 && value[i] != '\t' || value[i] == 127 {
				return usage("invalid " + name + " header value")
			}
		}
	}
	return nil
}

// detailMessage keeps the original detail untouched while extracting its sentence.
func detailMessage(raw json.RawMessage) string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	if list, ok := value.([]any); ok {
		if len(list) == 0 {
			return ""
		}
		value = list[0]
	}
	if obj, ok := value.(map[string]any); ok {
		for _, key := range []string{"message", "msg"} {
			if text, ok := obj[key].(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	return ""
}
