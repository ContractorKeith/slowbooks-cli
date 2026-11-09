package output

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
	"unicode"
)

func TestTTYEscapesEveryControl(t *testing.T) {
	for _, data := range []any{
		map[string]any{"key\x1b\a\b": "value\x1b\a\b"},
		[]map[string]any{{"head\x1b\a\b": "value\x1b\a\b"}},
	} {
		var out bytes.Buffer
		w := New(&out, io.Discard)
		w.TTY = true
		if err := w.Success(data, "", nil); err != nil {
			t.Fatal(err)
		}
		for _, r := range out.String() {
			if unicode.IsControl(r) && r != '\n' {
				t.Fatalf("raw control %U in %q", r, out.String())
			}
		}
		for _, escaped := range []string{`\u001b`, `\u0007`, `\u0008`} {
			if !strings.Contains(out.String(), escaped) {
				t.Fatal(out.String(), escaped)
			}
		}
	}
}

func TestExplicitColumnsAndWidePadding(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, io.Discard)
	w.TTY = true
	w.Columns = []string{"name", "id"}
	if err := w.Success([]map[string]any{{"aaa": "hidden", "name": "東京", "id": 1}, {"aaa": "hidden", "name": "abc", "id": 2}}, "", nil); err != nil {
		t.Fatal(err)
	}
	// 東京 uses four display cells, so its following column needs two spaces.
	want := "name  id\n東京  1\nabc   2\n"
	if out.String() != want {
		t.Fatalf("got %q want %q", out.String(), want)
	}
}

func TestJQBoundedResults(t *testing.T) {
	w := New(io.Discard, io.Discard)
	if err := w.SetJQ("repeat(1)", true); err != nil {
		t.Fatal(err)
	}
	if err := w.Success(nil, "", nil); ExitCode(err) != 2 || !strings.Contains(err.Error(), "100000 results") {
		t.Fatal(err)
	}
}

func TestJQCancellationAndTimeout(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		w := New(io.Discard, io.Discard)
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		defer cancel()
		w.SetContext(ctx, 20*time.Millisecond)
		if err := w.SetJQ("reduce range(1e9) as $i (0; . + $i)", true); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		err := w.Success(nil, "", nil)
		if ExitCode(err) != 2 || !strings.Contains(err.Error(), "time limit") || time.Since(start) > time.Second {
			t.Fatal(err, time.Since(start))
		}
	}
}
func TestJQByteCap(t *testing.T) {
	w := New(io.Discard, io.Discard)
	if err := w.SetJQ("repeat(.)", true); err != nil {
		t.Fatal(err)
	}
	err := w.Success(strings.Repeat("x", 1<<20), "", nil)
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatal(err)
	}
}
