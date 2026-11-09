package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestTTYViews(t *testing.T) {
	for _, key := range []string{"HOME", "SLOWBOOKS_CONFIG_DIR", "SLOWBOOKS_CACHE_DIR"} {
		t.Setenv(key, t.TempDir())
	}
	for _, tc := range []struct {
		data         any
		columns      []string
		want, absent string
	}{
		{[]map[string]any{{"id": 1, "name": "Acme", "nested": map[string]int{"n": 1}}}, []string{"id", "name"}, "Acme", "nested"},
		{[]any{map[string]any{"id": 1, "name": "line\nbreak", "nested": []int{1}}}, nil, "line\\nbreak", "nested"},
		{map[string]any{"name": "Acme"}, nil, "name: Acme", "\"ok\""},
		{[]int{1, 2}, nil, "[1,2]", "\"ok\""},
	} {
		var out bytes.Buffer
		w := New(&out, &out)
		w.TTY = true
		w.Columns = tc.columns
		if err := w.Success(tc.data, "", nil); err != nil || !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), tc.absent) {
			t.Fatal(out.String(), err)
		}
	}
}
