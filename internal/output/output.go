package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/itchyny/gojq"
	"golang.org/x/term"
)

// Writer owns command output, including help and diagnostics.
type Writer struct {
	Out     io.Writer
	Err     io.Writer
	JSON    bool
	Quiet   bool
	TTY     bool
	Columns []string
	jq      *gojq.Code
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
	if next == nil {
		next = []string{}
	}
	var value any = Success{true, data, summary, next}
	if w.Quiet {
		value = data
	}
	if w.jq != nil {
		return w.filter(value)
	}
	if w.Quiet || w.JSON || !w.TTY {
		return json.NewEncoder(w.Out).Encode(value)
	}
	return w.human(data)
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
		if e.Hint != "" {
			_, _ = fmt.Fprintln(w.Err, e.Hint)
		}
	}
	return ExitCode(e)
}

// Text is used for cobra's help and the fake server's startup URL.
func (w *Writer) Text(s string) error { _, err := io.WriteString(w.Out, s); return err }

// HelpWriter routes cobra help through the output package.
func (w *Writer) HelpWriter() io.Writer { return w.Out }
