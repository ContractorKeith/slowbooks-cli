package client

import (
	"context"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestR2CompanyRejectsAllControlsAndEdgeWhitespace(t *testing.T) {
	isolate(t)
	for _, company := range []string{"\tacme", "a\tb", " acme", "acme ", "\u00a0acme", "acme\u2003", "a\u0085b"} {
		_, err := New(context.Background(), Options{URL: "http://127.0.0.1:1", Company: company})
		if output.ExitCode(err) != 2 {
			t.Errorf("%q: %v", company, err)
		}
	}
}
