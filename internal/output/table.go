package output

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
)

// human normalizes typed values before choosing an object view or table.
func (w *Writer) human(data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var value any
	if err = json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if list, ok := value.([]any); ok && len(list) > 0 {
		rows := make([]map[string]any, len(list))
		for i, v := range list {
			var ok bool
			rows[i], ok = v.(map[string]any)
			if !ok {
				_, err = fmt.Fprintln(w.Out, string(raw))
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
			if _, err = fmt.Fprintf(w.Out, "%s: %s\n", key, cell(fields[key])); err != nil {
				return err
			}
		}
		return nil
	}
	_, err = fmt.Fprintln(w.Out, string(raw))
	return err
}

func (w *Writer) table(rows []map[string]any, columns []string) error {
	tw := tabwriter.NewWriter(w.Out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(columns, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		values := make([]string, len(columns))
		for i, k := range columns {
			values[i] = cell(row[k])
		}
		if _, err := fmt.Fprintln(tw, strings.Join(values, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}
func scalar(v any) bool {
	switch v.(type) {
	case nil, string, bool, float64:
		return true
	}
	return false
}
func cell(v any) string {
	if s, ok := v.(string); ok {
		return strings.NewReplacer("\t", "\\t", "\r", "\\r", "\n", "\\n", "\x1b", "\\u001b").Replace(s)
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}
func sortedKeys(fields map[string]any) []string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
