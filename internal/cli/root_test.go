package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

func isolate(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
}

func TestVersionGolden(t *testing.T) {
	isolate(t)
	var out, stderr bytes.Buffer
	if code := Run([]string{"version", "--json"}, &out, &stderr, Build{"v0.0.0-accept", "accept"}); code != 0 {
		t.Fatal(code, stderr.String())
	}
	expected, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "version.json"))
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(expected) {
		t.Fatalf("got %s; want %s", out.String(), expected)
	}
}

func TestEveryExitCodeThroughRoot(t *testing.T) {
	isolate(t)
	cases := []struct {
		code string
		exit int
	}{{"", 0}, {"error", 1}, {"usage", 2}, {"auth", 3}, {"refused", 4}, {"not_found", 5}, {"conflict", 6}, {"invalid", 7}, {"network", 8}, {"server", 8}}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			var out, stderr bytes.Buffer
			w := output.New(&out, &stderr)
			root := NewRoot(w, Build{"test", "test"})
			root.AddCommand(&cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, args []string) error {
				if tc.code == "" {
					return w.Success(map[string]int{"count": 1}, "one", nil)
				}
				return &output.Error{Code: tc.code, Message: "fixture", Detail: json.RawMessage(`[{"msg":"server detail"}]`)}
			}})
			if got := Execute(root, w, []string{"probe"}); got != tc.exit {
				t.Fatalf("exit %d; expected %d", got, tc.exit)
			}
			var envelope struct {
				OK    bool          `json:"ok"`
				Error *output.Error `json:"error"`
			}
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.OK != (tc.exit == 0) {
				t.Fatal(out.String())
			}
			if tc.exit != 0 && envelope.Error.Code != tc.code {
				t.Fatal(out.String())
			}
			if stderr.Len() != 0 {
				t.Fatal(stderr.String())
			}
		})
	}
}

func TestFlagsAndUsage(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{nil, {"nosuch"}, {"version", "extra"}, {"version", "--bad"}, {"version", "--timeout", "0"}, {"version", "--timeout", "3601"}, {"version", "--timeout", "invalid"}, {"version", "--jq", ".data"}} {
		var out bytes.Buffer
		if got := Run(args, &out, &out, Build{}); got != 2 {
			t.Fatalf("%v: exit %d", args, got)
		}
		var result struct {
			OK    bool
			Error *output.Error
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.OK || result.Error.Code != "usage" {
			t.Fatalf("%v: %s", args, out.String())
		}
	}
	var out bytes.Buffer
	if got := Run([]string{"--profile", "test", "version", "--company", "acme", "--timeout", "1", "--no-color", "--quiet"}, &out, &out, Build{"test", "test"}); got != 0 {
		t.Fatal(got)
	}
	if strings.Contains(out.String(), `"ok"`) || !strings.Contains(out.String(), `"verified_sbp"`) {
		t.Fatal(out.String())
	}
}

func TestUnexpectedErrorAndHumanOutput(t *testing.T) {
	isolate(t)
	var out, stderr bytes.Buffer
	w := output.New(&out, &stderr)
	w.TTY = true
	root := NewRoot(w, Build{})
	root.AddCommand(&cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return errors.New("unexpected") }})
	if got := Execute(root, w, []string{"probe"}); got != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "unexpected") {
		t.Fatalf("exit %d stdout %s stderr %s", got, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	w = output.New(&out, &stderr)
	w.TTY = true
	root = NewRoot(w, Build{"test", "test"})
	if got := Execute(root, w, []string{"version"}); got != 0 || strings.Contains(out.String(), `"ok"`) || !strings.Contains(out.String(), "version:") {
		t.Fatalf("exit %d: %s", got, out.String())
	}
}
