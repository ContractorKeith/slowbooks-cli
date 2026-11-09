package output

import (
	"bytes"
	"context"
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
		_ = w.consumeJQ(ctx, cancel, nil)
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
