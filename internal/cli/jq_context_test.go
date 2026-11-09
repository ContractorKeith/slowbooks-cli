package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func TestJQCommandContext(t *testing.T) {
	isolate(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	w := output.New(&out, &out)
	root := NewRoot(w, Build{})
	root.SetContext(ctx)
	code := Execute(root, w, []string{"version", "--jq", "reduce range(1e9) as $i (0; . + $i)"})
	if code != 2 || !strings.Contains(out.String(), "context cancelled") {
		t.Fatal(code, out.String())
	}
}
func TestJQCommandTimeout(t *testing.T) {
	isolate(t)
	var out bytes.Buffer
	w := output.New(&out, &out)
	root := NewRoot(w, Build{})
	start := time.Now()
	code := Execute(root, w, []string{"version", "--timeout", "1", "--jq", "reduce range(1e9) as $i (0; . + $i)"})
	if code != 2 || !strings.Contains(out.String(), "time limit") || time.Since(start) > 5*time.Second {
		t.Fatal(code, out.String(), time.Since(start))
	}
}
