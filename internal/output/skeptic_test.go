package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestSkepticStderrControls(t *testing.T) {
	var out, stderr bytes.Buffer
	w := New(&out, &stderr)
	w.TTY = true
	w.Failure(&Error{Code: "refused", Message: "denied\x1b\a\n", Hint: "hint\x1b\a", Detail: "detail\x1b"})
	if strings.ContainsAny(stderr.String(), "\x1b\a") || !strings.Contains(stderr.String(), `\u001b`) {
		t.Fatal(stderr.String())
	}
}
func TestSkepticJSONNoHTMLEscapes(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &out)
	if err := w.Success("<>&", "", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<>&") {
		t.Fatal(out.String())
	}
	out.Reset()
	w.Failure(&Error{Message: "<>&"})
	if !strings.Contains(out.String(), "<>&") {
		t.Fatal(out.String())
	}
}
func TestSkepticTTYTrimSummary(t *testing.T) {
	var out, stderr bytes.Buffer
	w := New(&out, &stderr)
	w.TTY = true
	if err := w.Success([]any{map[string]any{"id": 1}}, "1 of 2 shown; --all for everything", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "1 of 2 shown; --all for everything") {
		t.Fatal(stderr.String())
	}
}

func TestSkepticJQNoHTMLEscapes(t *testing.T) {
	var out bytes.Buffer
	w := New(&out, &out)
	if err := w.SetJQ(".data", true); err != nil {
		t.Fatal(err)
	}
	if err := w.Success("<>&", "", nil); err != nil {
		t.Fatal(err)
	}
	if out.String() != "\"<>&\"\n" {
		t.Fatal(out.String())
	}
}
