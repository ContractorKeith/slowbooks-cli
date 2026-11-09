package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"golang.org/x/term"
)

// Writer owns command output, including help and diagnostics.
type Writer struct {
	Out   io.Writer
	Err   io.Writer
	JSON  bool
	Quiet bool
	TTY   bool
}

// New detects a terminal only for actual file descriptors.
func New(out, err io.Writer) *Writer {
	w := &Writer{Out: out, Err: err}
	if f, ok := out.(*os.File); ok {
		w.TTY = term.IsTerminal(int(f.Fd()))
	}
	return w
}

type Success struct {
	OK      bool     `json:"ok"`
	Data    any      `json:"data"`
	Summary string   `json:"summary"`
	Next    []string `json:"next"`
}

// Success renders data only in quiet mode; otherwise it emits an envelope or view.
func (w *Writer) Success(data any, summary string, next []string) error {
	if w.Quiet {
		return json.NewEncoder(w.Out).Encode(data)
	}
	if w.JSON || !w.TTY {
		if next == nil {
			next = []string{}
		}
		return json.NewEncoder(w.Out).Encode(Success{true, data, summary, next})
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		_, err = fmt.Fprintln(w.Out, string(encoded))
		return err
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := fmt.Fprintf(w.Out, "%s: %v\n", key, fields[key]); err != nil {
			return err
		}
	}
	return nil
}

// Failure sends JSON errors to stdout and human diagnostics to stderr.
func (w *Writer) Failure(err error) int {
	e := AsError(err)
	if w.JSON || !w.TTY {
		if writeErr := json.NewEncoder(w.Out).Encode(struct {
			OK    bool   `json:"ok"`
			Error *Error `json:"error"`
		}{false, e}); writeErr != nil {
			return 1
		}
	} else {
		_, _ = fmt.Fprintln(w.Err, e.Message)
	}
	return ExitCode(e)
}

// Text is used for cobra's help and the fake server's startup URL.
func (w *Writer) Text(s string) error { _, err := io.WriteString(w.Out, s); return err }

// HelpWriter routes cobra help through the output package.
func (w *Writer) HelpWriter() io.Writer { return w.Out }
