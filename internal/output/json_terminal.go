package output

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf16"
	"unicode/utf8"
)

// terminalJSONWriter adds JSON Unicode escapes, not human-format text escapes.
// Decoding preserves the original strings, including supplementary-plane runes.
type terminalJSONWriter struct{ out io.Writer }

func (w terminalJSONWriter) Write(p []byte) (int, error) {
	var encoded bytes.Buffer
	last := 0
	for i, r := range string(p) {
		// JSON encoding already escapes C0 inside strings; retain its newline separator.
		if r < 0x7f || !needsTerminalEscape(r) {
			continue
		}
		encoded.Write(p[last:i])
		if r <= 0xffff {
			fmt.Fprintf(&encoded, `\u%04x`, r)
		} else {
			high, low := utf16.EncodeRune(r)
			fmt.Fprintf(&encoded, `\u%04x\u%04x`, high, low)
		}
		last = i + utf8.RuneLen(r)
	}
	if last == 0 {
		return w.out.Write(p)
	}
	encoded.Write(p[last:])
	n, err := w.out.Write(encoded.Bytes())
	if err != nil {
		return 0, err
	}
	if n != encoded.Len() {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}
