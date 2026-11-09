package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Warning escapes untrusted diagnostic text with the table escaper.
func (w *Writer) Warning(message string) error {
	_, err := fmt.Fprintln(w.Err, "warning: "+escapeControls(message))
	return err
}
func jsonEncoder(out io.Writer, terminal bool) *json.Encoder {
	if terminal {
		out = terminalJSONWriter{out}
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc
}
