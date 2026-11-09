package output

import (
	"bytes"
	"strings"
	"testing"
	"unicode"
)

func TestR2EveryTTYShapeEscapesControlsAndFormat(t *testing.T) {
	bad := "a\x1b\x7f\u009b\u0085\u202e\u2066\u200b\U000e0001b"
	for _, value := range []any{bad, []any{bad}, map[string]any{bad: bad}, []any{map[string]any{"name": bad}}, map[string]any{"nested": map[string]any{"value": bad}}} {
		var out, stderr bytes.Buffer
		w := New(&out, &stderr)
		w.TTY = true
		if err := w.Success(value, "", nil); err != nil {
			t.Fatal(err)
		}
		w.Failure(&Error{Code: "refused", Message: bad, Hint: bad})
		if err := w.Warning(bad); err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{out.String(), stderr.String()} {
			for _, r := range text {
				if (unicode.IsControl(r) && r != '\n') || unicode.Is(unicode.Cf, r) {
					t.Errorf("unescaped %U in %q", r, text)
				}
			}
			for _, want := range []string{`\u009b`, `\u202e`, `\u2066`, `\u200b`} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %s in %q", want, text)
				}
			}
		}
	}
}
