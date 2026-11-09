package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func agentReference(t *testing.T, args ...string) agentHelp {
	t.Helper()
	var out bytes.Buffer
	if code := Run(args, &out, &out, Build{"v1.2.3", "test"}); code != 0 {
		t.Fatalf("exit %d: %s", code, &out)
	}
	var env struct {
		OK   bool
		Data agentHelp
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil || !env.OK {
		t.Fatalf("%s: %v", &out, err)
	}
	return env.Data
}

func TestAgentCoversTree(t *testing.T) {
	isolate(t)
	ref := agentReference(t, "commands", "--json")
	if !reflect.DeepEqual(ref, agentReference(t, "--help", "--agent")) {
		t.Fatal("agent help and commands differ")
	}
	indexed := map[string]agentCommand{}
	for _, item := range ref.Commands {
		indexed[item.Name] = item
	}
	root := NewRoot(output.New(&bytes.Buffer{}, &bytes.Buffer{}), Build{})
	var walk func(*cobra.Command)
	count := 0
	walk = func(cmd *cobra.Command) {
		count++
		cmd.InitDefaultHelpFlag()
		item, ok := indexed[cmd.CommandPath()]
		if !ok {
			t.Fatalf("missing command %s", cmd.CommandPath())
		}
		if item.Args != strings.TrimSpace(strings.TrimPrefix(cmd.Use, cmd.Name())) {
			t.Fatal("argument mismatch", item)
		}
		flags := map[string]agentFlag{}
		for _, flag := range item.Flags {
			flags[flag.Name] = flag
		}
		check := func(f *pflag.Flag) {
			flag, ok := flags[f.Name]
			if !ok || flag.Type != f.Value.Type() || flag.Default != f.DefValue {
				t.Fatalf("%s missing/mismatched flag %s", cmd.CommandPath(), f.Name)
			}
		}
		cmd.LocalFlags().VisitAll(check)
		cmd.InheritedFlags().VisitAll(check)
		if len(item.Examples) == 0 {
			t.Fatal("no example", item.Name)
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	if count != len(ref.Commands) {
		t.Fatal("extra command")
	}
	if !reflect.DeepEqual(ref.ExitCodes, output.ExitCodes()) {
		t.Fatal("exit codes drifted")
	}
	for _, row := range append(commands.Reads(), commands.Writes()...) {
		item := indexed["slowbooks "+row.Name]
		if item.FileOutput != row.Download || (item.Access == "write") != row.Write {
			t.Fatal("incorrect metadata", item)
		}
		if row.Path != "" && (item.Operation == nil || item.Operation.Method != row.Method || item.Operation.Path != row.Path) {
			t.Fatal("operation mismatch", item)
		}
		for _, hint := range row.Next {
			assertRealCommand(t, indexed, hint)
		}
		if strings.HasSuffix(row.Name, " record") {
			found := false
			for _, f := range item.Flags {
				if f.Name == "key" && f.Required && item.Keyed {
					found = true
				}
			}
			if !found {
				t.Fatal("missing payment key requirement")
			}
		}
	}
}

func assertRealCommand(t *testing.T, indexed map[string]agentCommand, line string) {
	t.Helper()
	fields := strings.Fields(line)
	for n := len(fields); n > 0; n-- {
		name := strings.Join(fields[:n], " ")
		if item, ok := indexed[name]; ok && (item.Runnable || line == "slowbooks --help --agent") {
			if n == 1 && line != "slowbooks --help --agent" {
				t.Fatalf("unrecognized command: %s", line)
			}
			return
		}
	}
	t.Fatalf("unrecognized command: %s", line)
}

func TestSkillCommandsAndFrontmatter(t *testing.T) {
	isolate(t)
	raw, err := os.ReadFile("../../skills/slowbooks-cli/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(raw), "---\n", 3)
	if len(parts) != 3 || parts[0] != "" {
		t.Fatal("missing frontmatter")
	}
	// The skill uses the YAML plain-scalar subset: one name and description.
	front := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(parts[1]), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok || strings.ContainsAny(value, "\n{}[]") {
			t.Fatal("invalid scalar frontmatter", line)
		}
		front[key] = value
	}
	if front["name"] != "slowbooks-cli" || len(front["name"]) > 64 || len(front["description"]) == 0 || len(front["description"]) > 1024 {
		t.Fatal(front)
	}
	if len(strings.Split(string(raw), "\n")) >= 150 {
		t.Fatal("skill too long")
	}
	indexed := map[string]agentCommand{}
	for _, item := range agentReference(t, "commands", "--json").Commands {
		indexed[item.Name] = item
	}
	count := 0
	for _, line := range strings.Split(parts[2], "\n") {
		if strings.HasPrefix(line, "slowbooks ") {
			count++
			assertRealCommand(t, indexed, line)
		}
	}
	if count == 0 {
		t.Fatal("no examples checked")
	}
}
