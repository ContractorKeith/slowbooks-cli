package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode"
)

func TestReviewLegitimateJoinersAndSoftHyphensStayReadable(t *testing.T) {
	name := "Fam 👨‍👩‍👧 LLC \u200cname\u00adend"
	var out, stderr bytes.Buffer
	w := New(&out, &stderr)
	w.TTY = true
	if err := w.Success([]any{map[string]any{"name": name}}, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := w.Warning(name); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), name) || !strings.Contains(stderr.String(), name) {
		t.Fatal(out.String(), stderr.String())
	}
}
func TestReviewTTYJSONEscapesWithoutChangingStringValues(t *testing.T) {
	value := "a\x1b\x7f\u009b\u202e\u2066\U000e0001 Fam 👨‍👩‍👧 LLC \u200c\u00ad"
	for _, jq := range []bool{false, true} {
		var out bytes.Buffer
		w := New(&out, &out)
		w.TTY = true
		w.JSON = true
		if jq {
			if err := w.SetJQ(".data", true); err != nil {
				t.Fatal(err)
			}
		}
		if err := w.Success(value, "", nil); err != nil {
			t.Fatal(err)
		}
		var decoded any
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatal(err, out.String())
		}
		if !jq {
			decoded = decoded.(map[string]any)["data"]
		}
		if decoded != value {
			t.Fatalf("JSON value changed: %q", decoded)
		}
		for _, r := range out.String() {
			legit := r == '\u200c' || r == '\u200d' || r == '\u00ad'
			if !legit && (unicode.IsControl(r) && r != '\n' || unicode.Is(unicode.Cf, r)) {
				t.Errorf("raw %U in %q", r, out.String())
			}
		}
		if !strings.Contains(out.String(), "Fam 👨‍👩‍👧 LLC") {
			t.Fatal("legitimate emoji changed", out.String())
		}
	}
}
