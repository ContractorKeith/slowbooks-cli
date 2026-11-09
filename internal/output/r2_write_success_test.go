package output

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestR2WriteFilterBoundsPreserveSuccess(t *testing.T) {
	for _, expr := range []string{"repeat(1)", "[range(1e9)]|length"} {
		var out, stderr bytes.Buffer
		w := New(&out, &stderr)
		w.Quiet = true
		w.jqMemoryLimit = 1 << 20
		if err := w.SetJQ(expr, true); err != nil {
			t.Fatal(err)
		}
		if err := w.SuccessWrite(map[string]int{"id": 43}, "created", nil); err != nil {
			t.Fatal(err)
		}
		var envelope Success
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil || !envelope.OK || !strings.Contains(stderr.String(), "--jq failed after successful write") {
			t.Fatal(out.String(), stderr.String(), err)
		}
	}
}
func TestR2StuckWriteFilterExitsWithSuccess(t *testing.T) {
	if os.Getenv("SBP_TEST_R2_WRITE_STUCK") == "1" {
		held := make([]byte, 4<<20)
		defer runtime.KeepAlive(held)
		var filtered bytes.Buffer
		w := New(&filtered, os.Stderr)
		w.jqMemoryLimit = 1 << 20
		w.writeResult = &writeResult{Success{true, map[string]int{"id": 44}, "created", nil}, os.Stdout}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = w.consumeJQ(ctx, cancel, nil, 0)
		t.Fatal("stuck write filter returned")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestR2StuckWriteFilterExitsWithSuccess$")
	cmd.Env = append(os.Environ(), "SBP_TEST_R2_WRITE_STUCK=1", "HOME="+t.TempDir(), "SLOWBOOKS_CONFIG_DIR="+t.TempDir(), "SLOWBOOKS_CACHE_DIR="+t.TempDir())
	raw, err := cmd.CombinedOutput()
	if err != nil || ctx.Err() != nil {
		t.Fatalf("%v %s", err, raw)
	}
	lines := strings.SplitN(string(raw), "\n", 2)
	if len(lines) != 2 || lines[0] != `{"ok":true,"data":{"id":44},"summary":"created","next":[]}` || !strings.Contains(lines[1], "--jq failed after successful write") {
		t.Fatal(string(raw))
	}
}
