package output

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestJQMemoryLimitCancelsIntermediateArray(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &out)
	w.SetContext(context.Background(), 250*time.Millisecond)
	w.jqMemoryLimit = 1 << 20
	if err := w.SetJQ("[range(1e9)]|length", true); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err := w.Success(nil, "", nil)
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "--jq used more than 1 MiB") || time.Since(start) > time.Second {
		t.Fatal(err, time.Since(start))
	}
	if out.Len() != 0 {
		t.Fatal(out.String())
	}
}

func TestJQMemoryWatchdogExitsForStuckEvaluation(t *testing.T) {
	if os.Getenv("SBP_TEST_JQ_STUCK") == "1" {
		held := make([]byte, 4<<20)
		defer runtime.KeepAlive(held)
		w := New(os.Stdout, os.Stderr)
		w.jqMemoryLimit = 1 << 20
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = w.consumeJQ(ctx, cancel, nil, 0)
		t.Fatal("stuck evaluator returned without exiting")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestJQMemoryWatchdogExitsForStuckEvaluation$")
	cmd.Env = append(os.Environ(), "SBP_TEST_JQ_STUCK=1", "HOME="+t.TempDir(), "SLOWBOOKS_CONFIG_DIR="+t.TempDir(), "SLOWBOOKS_CACHE_DIR="+t.TempDir())
	raw, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || ctx.Err() != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	want := `{"ok":false,"error":{"code":"usage","message":"--jq used more than 1 MiB","retryable":false,"hint":"","detail":null}}` + "\n"
	if string(raw) != want {
		t.Fatalf("got %q", raw)
	}
}

func TestJQExistingFortyMiBResponseDoesNotSpendEvaluationBudget(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &out)
	w.jqMemoryLimit = 8 << 20
	if err := w.SetJQ(".data[].id", true); err != nil {
		t.Fatal(err)
	}
	rows := []any{map[string]any{"id": json.Number("9007199254740993"), "padding": strings.Repeat("x", 40<<20)}}
	if err := w.Success(rows, "1 row", nil); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9007199254740993\n" {
		t.Fatal(out.String())
	}
}

func TestJQTypedMetadataPreservesJSONShapeAndNumbers(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &out)
	if err := w.SetJQ(`.data | {id,body,omit:has("omit"),hidden:has("hidden")}`, true); err != nil {
		t.Fatal(err)
	}
	data := struct {
		ID     json.Number     `json:"id"`
		Body   json.RawMessage `json:"body"`
		Omit   string          `json:"omit,omitempty"`
		Hidden string          `json:"-"`
	}{ID: json.Number("9007199254740993"), Body: json.RawMessage(`{"id":9007199254740995}`), Hidden: "private"}
	if err := w.Success(data, "", nil); err != nil {
		t.Fatal(err)
	}
	want := `{"body":{"id":9007199254740995},"hidden":false,"id":9007199254740993,"omit":false}` + "\n"
	if out.String() != want {
		t.Fatal(out.String())
	}
}

func TestJQGrowthBudgetIgnoresExistingHeapDuringEvaluation(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &out)
	w.jqMemoryLimit = 32 << 20
	w.SetContext(context.Background(), 200*time.Millisecond)
	if err := w.SetJQ("reduce range(1e9) as $i (0; .)", true); err != nil {
		t.Fatal(err)
	}
	data := strings.Repeat("x", 40<<20)
	defer runtime.KeepAlive(data)
	err := w.Success(data, "", nil)
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "time limit") {
		t.Fatal(err)
	}
}
