package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestHumanWriteFailureIncludesSafetyHint(t *testing.T) {
	isolate(t)
	var out, stderr bytes.Buffer
	w := output.New(&out, &stderr)
	w.TTY = true
	root := NewRoot(w, Build{})
	root.AddCommand(&cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error {
		return &output.Error{Code: "network", Message: "request timed out", Hint: "check the books before running this write again"}
	}})
	if got := Execute(root, w, []string{"probe"}); got != 8 {
		t.Fatal(got)
	}
	if out.Len() != 0 || stderr.String() != "request timed out\ncheck the books before running this write again\n" {
		t.Fatalf("stdout %q stderr %q", out.String(), stderr.String())
	}
}

func TestWriterDefaultsSurviveFlagRegistration(t *testing.T) {
	isolate(t)
	for _, tc := range []struct{ json, quiet bool }{{true, false}, {false, true}} {
		var out, stderr bytes.Buffer
		w := output.New(&out, &stderr)
		w.TTY = true
		w.JSON, w.Quiet = tc.json, tc.quiet
		root := NewRoot(w, Build{"test", "test"})
		if w.JSON != tc.json || w.Quiet != tc.quiet {
			t.Fatal("registering flags cleared writer defaults")
		}
		if got := Execute(root, w, []string{"version"}); got != 0 {
			t.Fatal(got)
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if tc.quiet {
			if result["version"] != "test" || result["ok"] != nil {
				t.Fatal(result)
			}
		} else if result["ok"] != true {
			t.Fatal(result)
		}
	}
}

func TestJSONFlagAppliesToValidationErrors(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{{"version", "--json", "--timeout", "0"}, {"version", "--json", "extra"}, {"version", "--json", "--bad"}} {
		var out, stderr bytes.Buffer
		w := output.New(&out, &stderr)
		w.TTY = true
		root := NewRoot(w, Build{})
		if got := Execute(root, w, args); got != 2 {
			t.Fatal(got)
		}
		var result struct {
			OK    bool
			Error *output.Error
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Error == nil || result.Error.Code != "usage" || stderr.Len() != 0 {
			t.Fatalf("stdout %q stderr %q", out.String(), stderr.String())
		}
	}
}
