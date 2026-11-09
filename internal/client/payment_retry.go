package client

import (
	"net/url"
	"strings"
)

// These server operations ignore Idempotency-Key; its presence cannot justify
// replay advice after a response is lost or a gateway fails.
func supportsKeyReplay(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	path = strings.ToLower(u.Path)
	if path == "/api/batch-payments" {
		return false
	}
	parts := strings.Split(path, "/")
	return !(len(parts) == 5 && parts[0] == "" && parts[1] == "api" && (parts[2] == "payments" || parts[2] == "bill-payments") && parts[4] == "replace")
}
