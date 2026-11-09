package client

import (
	"strings"
	"unicode"
)

// ValidateKey prevents header normalization and local failures from changing retry advice.
func ValidateKey(key string) error {
	if key == "" || len(key) > 200 || strings.TrimSpace(key) != key || strings.ContainsFunc(key, unicode.IsControl) {
		return usage("--key must be 1–200 bytes with no control characters or leading/trailing whitespace")
	}
	return nil
}

// ValidatePathParam keeps one placeholder inside its original path segment.
func ValidatePathParam(value string) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\%{}") || strings.ContainsFunc(value, unicode.IsControl) {
		return usage("path parameters must be non-empty single segments without traversal or ambiguous encodings")
	}
	return nil
}
