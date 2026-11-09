package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestAgentRequiresHelpAndShowsSubtree(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{{"--agent"}, {"version", "--agent"}, {"commands", "--agent"}, {"invoices", "--agent"}, {"invoices", "list", "--agent"}, {"help", "invoices", "--agent"}} {
		var out bytes.Buffer
		if code := Run(args, &out, &out, Build{}); code != 2 {
			t.Fatalf("%v: %d %s", args, code, &out)
		}
		if !strings.Contains(out.String(), "--agent requires --help") {
			t.Fatal(args, &out)
		}
	}
	for _, path := range [][]string{{"invoices"}, {"invoices", "list"}} {
		ref := agentReference(t, append(path, "--help", "--agent")...)
		prefix := "sbp " + strings.Join(path, " ")
		for _, item := range ref.Commands {
			if item.Name != prefix && !strings.HasPrefix(item.Name, prefix+" ") {
				t.Fatal("outside subtree", item.Name, prefix)
			}
		}
		if len(ref.Commands) == 0 || ref.Commands[0].Name != prefix {
			t.Fatal(ref)
		}
		if len(path) == 2 && len(ref.Commands) != 1 {
			t.Fatal(ref)
		}
	}
}

func TestDescribeTreeDoesNotMutateFlags(t *testing.T) {
	isolate(t)
	root := NewRoot(output.New(io.Discard, io.Discard), Build{})
	snapshot := func() map[string][]string {
		result := map[string][]string{}
		var walk func(*cobra.Command)
		walk = func(cmd *cobra.Command) {
			cmd.Flags().VisitAll(func(f *pflag.Flag) { result[cmd.CommandPath()] = append(result[cmd.CommandPath()], f.Name) })
			for _, child := range cmd.Commands() {
				walk(child)
			}
		}
		walk(root)
		return result
	}
	before := snapshot()
	describeTree(root)
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("command flags mutated", before, after)
	}
}

func TestHumanHelpAfterCommandsHasNoDuplicateGlobals(t *testing.T) {
	isolate(t)
	for _, path := range [][]string{nil, {"invoices"}} {
		var out bytes.Buffer
		w := output.New(&out, &out)
		root := NewRoot(w, Build{})
		if code := Execute(root, w, []string{"commands", "--json"}); code != 0 {
			t.Fatal(code, &out)
		}
		out.Reset()
		args := append(path, "--help")
		if code := Execute(root, w, args); code != 0 {
			t.Fatal(code, &out)
		}
		for _, name := range []string{"agent", "company", "jq", "json", "no-color", "profile", "quiet", "timeout"} {
			if count := strings.Count(out.String(), "--"+name+" "); count != 1 {
				t.Fatalf("%s appears %d times in %s", name, count, &out)
			}
		}
		var expected bytes.Buffer
		if code := Run(args, &expected, &expected, Build{}); code != 0 || expected.String() != out.String() {
			t.Fatalf("help changed: %d\n%s\n%s", code, &expected, &out)
		}
	}
}

func TestManifestRejectsPrereleaseBuild(t *testing.T) {
	isolate(t)
	var out bytes.Buffer
	if code := Run([]string{"commands", "--manifest"}, &out, &out, Build{"v0.1.0-rc.1", "test"}); code != 2 {
		t.Fatal(code, &out)
	}
	var env struct{ Error *output.Error }
	if err := json.Unmarshal(out.Bytes(), &env); err != nil || env.Error == nil || env.Error.Code != "usage" {
		t.Fatal(&out, err)
	}
}

type failedHelpWriter struct{}

func (failedHelpWriter) Write([]byte) (int, error) { return 0, errors.New("help output failed") }
func TestAgentHelpOutputFailure(t *testing.T) {
	isolate(t)
	if code := Run([]string{"--help", "--agent"}, failedHelpWriter{}, io.Discard, Build{}); code != 1 {
		t.Fatal(code)
	}
}
