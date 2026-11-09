package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// human normalizes typed values before choosing an object view or table.
func (w *Writer) human(data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&value); err != nil {
		return err
	}
	if list, ok := value.([]any); ok && len(list) > 0 {
		rows := make([]map[string]any, len(list))
		for i, v := range list {
			var ok bool
			rows[i], ok = v.(map[string]any)
			if !ok {
				_, err = fmt.Fprintln(w.Out, escapeControls(string(raw)))
				return err
			}
		}
		columns := w.Columns
		if len(columns) == 0 {
			fields := map[string]any{}
			for _, row := range rows {
				for k, v := range row {
					if scalar(v) {
						fields[k] = v
					}
				}
			}
			columns = sortedKeys(fields)
			if len(columns) > 5 {
				columns = columns[:5]
			}
		}
		if len(columns) > 0 {
			return w.table(rows, columns)
		}
	}
	if fields, ok := value.(map[string]any); ok {
		for _, key := range sortedKeys(fields) {
			if _, err = fmt.Fprintf(w.Out, "%s: %s\n", escapeControls(key), cell(fields[key])); err != nil {
				return err
			}
		}
		return nil
	}
	_, err = fmt.Fprintln(w.Out, escapeControls(string(raw)))
	return err
}

func (w *Writer) table(rows []map[string]any, columns []string) error {
	cells := make([][]string, 1, len(rows)+1)
	cells[0] = make([]string, len(columns))
	for i, key := range columns {
		cells[0][i] = escapeControls(key)
	}
	for _, row := range rows {
		values := make([]string, len(columns))
		for i, key := range columns {
			values[i] = cell(row[key])
		}
		cells = append(cells, values)
	}
	widths := make([]int, len(columns))
	for _, row := range cells {
		for i, value := range row {
			widths[i] = max(widths[i], displayWidth(value))
		}
	}
	for _, row := range cells {
		for i, value := range row {
			if _, err := fmt.Fprint(w.Out, value); err != nil {
				return err
			}
			if i < len(row)-1 {
				if _, err := fmt.Fprint(w.Out, strings.Repeat(" ", widths[i]-displayWidth(value)+2)); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprintln(w.Out); err != nil {
			return err
		}
	}
	return nil
}
func scalar(v any) bool {
	switch v.(type) {
	case nil, string, bool, float64, json.Number:
		return true
	}
	return false
}
func cell(v any) string {
	if s, ok := v.(string); ok {
		return escapeControls(s)
	}
	raw, _ := json.Marshal(v)
	return escapeControls(string(raw))
}
func sortedKeys(fields map[string]any) []string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func escapeControls(s string) string {
	var b strings.Builder
	for _, r := range s {
		if needsTerminalEscape(r) {
			switch r {
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if r > 0xffff {
					fmt.Fprintf(&b, `\U%08x`, r)
				} else {
					fmt.Fprintf(&b, `\u%04x`, r)
				}
			}
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Preserve joining controls and soft hyphens used in names and emoji. Bidi and
// all other format/control characters remain escaped in terminal output.
func needsTerminalEscape(r rune) bool {
	if r == '\u200c' || r == '\u200d' || r == '\u00ad' {
		return false
	}
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}
