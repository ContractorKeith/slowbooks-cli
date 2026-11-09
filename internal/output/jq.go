package output

import (
	"context"
	"time"

	"github.com/itchyny/gojq"
)

// SetJQ compiles before any command side effects, including an explicitly empty filter.
func (w *Writer) SetJQ(expr string, enabled bool) error {
	w.jq = nil
	if !enabled {
		return nil
	}
	query, err := gojq.Parse(expr)
	if err != nil {
		return &Error{Code: "usage", Message: "--jq: " + err.Error()}
	}
	w.jq, err = gojq.Compile(query)
	if err != nil {
		return &Error{Code: "usage", Message: "--jq: " + err.Error()}
	}
	return nil
}

func (w *Writer) filter(value any) error {
	ctx := w.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := w.timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	input, err := jqInput(value)
	if err != nil {
		return err
	}
	baseline := jqHeapUsage()
	results := make(chan jqResult)
	go streamJQ(ctx, w.jq, input, results, w.TTY)
	return w.consumeJQ(ctx, cancel, results, baseline)
}

// SetContext bounds jq evaluation by the command's cancellation and timeout.
func (w *Writer) SetContext(ctx context.Context, timeout time.Duration) {
	w.ctx, w.timeout = ctx, timeout
}
func jqBound(message string) error { return &Error{Code: "usage", Message: "--jq: " + message} }
