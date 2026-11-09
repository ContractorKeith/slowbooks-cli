package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var input any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&input); err != nil {
		return err
	}
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
	iter := w.jq.RunWithContext(ctx, input)
	remaining := 64 << 20
	for count := 0; ; count++ {
		if ctx.Err() != nil {
			return jqBound("context cancelled or time limit exceeded")
		}
		value, ok := iter.Next()
		if !ok {
			if ctx.Err() != nil {
				return jqBound("context cancelled or time limit exceeded")
			}
			return nil
		}
		if ctx.Err() != nil {
			return jqBound("context cancelled or time limit exceeded")
		}
		if count >= 100000 {
			return jqBound("output exceeds 100000 results")
		}
		if err, ok := value.(error); ok {
			return &Error{Code: "usage", Message: fmt.Sprintf("--jq: %v", err)}
		}
		var buf bytes.Buffer
		if err := jsonEncoder(&buf).Encode(value); err != nil {
			return err
		}
		encoded := buf.Bytes()
		if len(encoded) > remaining {
			return jqBound("output exceeds 64 MiB")
		}
		remaining -= len(encoded)
		if _, err := w.Out.Write(encoded); err != nil {
			return err
		}
	}
}

// SetContext bounds jq evaluation by the command's cancellation and timeout.
func (w *Writer) SetContext(ctx context.Context, timeout time.Duration) {
	w.ctx, w.timeout = ctx, timeout
}
func jqBound(message string) error { return &Error{Code: "usage", Message: "--jq: " + message} }
