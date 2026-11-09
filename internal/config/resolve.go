package config

import (
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

type Flags struct {
	Profile, Company, URL                      string
	Timeout                                    int
	ProfileSet, CompanySet, URLSet, TimeoutSet bool
}
type Connection struct {
	Profile   string
	URL       string
	Company   string
	TokenFile string
	Timeout   time.Duration
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Timeout applies flag and environment precedence, then validates the range.
func Timeout(f Flags) (time.Duration, error) {
	n := 120
	if v := os.Getenv("SLOWBOOKS_TIMEOUT_SECONDS"); v != "" && !f.TimeoutSet {
		var e error
		n, e = strconv.Atoi(v)
		if e != nil {
			return 0, timeoutError()
		}
	}
	if f.TimeoutSet {
		n = f.Timeout
	}
	if n < 1 || n > 3600 {
		return 0, timeoutError()
	}
	return time.Duration(n) * time.Second, nil
}
func timeoutError() error {
	return &output.Error{Code: "usage", Message: "timeout must be 1–3600 seconds"}
}
func Selected(f Flags, s Store) string {
	if f.ProfileSet {
		return f.Profile
	}
	return first(os.Getenv("SLOWBOOKS_PROFILE"), s.Default)
}

// Resolve builds a connection without reading its token contents.
func Resolve(f Flags) (Connection, error) {
	var c Connection
	var e error
	c.Timeout, e = Timeout(f)
	if e != nil {
		return c, e
	}
	// Complete environment connections do not need a readable profile file.
	environment := envOnly(f)
	s := Store{}
	if !environment {
		s, e = Load()
		if e != nil {
			return c, e
		}
	}
	c.Profile = Selected(f, s)
	p, ok := s.Profiles[c.Profile]
	if !environment && !ok {
		return c, missing()
	}
	c.URL = first(os.Getenv("SLOWBOOKS_URL"), p.URL)
	if f.URLSet {
		c.URL = f.URL
	}
	c.Company = first(os.Getenv("SLOWBOOKS_COMPANY"), p.Company)
	if f.CompanySet {
		c.Company = f.Company
	}
	c.TokenFile = first(os.Getenv("SLOWBOOKS_TOKEN_FILE"), p.TokenFile)
	if c.URL == "" || c.TokenFile == "" {
		return c, missing()
	}
	return c, nil
}

// ValidateToken accepts a single token with an optional final line ending.
func ValidateToken(b []byte) (string, error) {
	s := string(b)
	if strings.HasSuffix(s, "\n") {
		s = strings.TrimSuffix(s, "\n")
		s = strings.TrimSuffix(s, "\r")
	}
	if len(s) <= 4 || !strings.HasPrefix(s, "sbp_") || strings.ContainsFunc(s, unicode.IsSpace) {
		return "", invalidToken()
	}
	return s, nil
}
func invalidToken() error {
	return &output.Error{Code: "auth", Message: "invalid token: expected one line starting with sbp_ and no whitespace", Hint: "run sbp auth login"}
}
