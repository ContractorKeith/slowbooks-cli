package client

import (
	"regexp"
	"strings"
	"unicode"
)

var paymentKey = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// ValidateKey prevents header normalization and local failures from changing retry advice.
func ValidateKey(key string) error {
	if !paymentKey.MatchString(key) {
		return usage("--key must match [A-Za-z0-9._:-]{1,128}")
	}
	return nil
}

// ValidatePathParam keeps one placeholder inside its original path segment.
func ValidatePathParam(value string) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\%{}?#") || strings.ContainsFunc(value, unicode.IsControl) {
		return usage("path parameters must be non-empty single segments without traversal or ambiguous encodings")
	}
	return nil
}
