package output

import (
	"bytes"
	"io"
)

type writeResult struct {
	envelope Success
	out      io.Writer
}

// SuccessWrite retains the committed outcome if a runtime jq filter fails.
// Buffering bounded jq output prevents partial projections hiding the envelope.
func (w *Writer) SuccessWrite(data any, summary string, next []string) error {
	if w.jq == nil {
		return w.Success(data, summary, next)
	}
	var buffer bytes.Buffer
	filtered := *w
	filtered.Out = &buffer
	filtered.writeResult = &writeResult{Success{true, data, summary, next}, w.Out}
	if err := filtered.Success(data, summary, next); err != nil {
		return filtered.recoverWriteFilter(err)
	}
	_, err := w.Out.Write(buffer.Bytes())
	return err
}

func (w *Writer) recoverWriteFilter(err error) error {
	result := w.writeResult
	original := *w
	original.Out = result.out
	original.jq = nil
	original.JSON = true
	original.Quiet = false
	if e := original.Success(result.envelope.Data, result.envelope.Summary, result.envelope.Next); e != nil {
		return e
	}
	return original.Warning("--jq failed after successful write: " + err.Error() + "; returned the unfiltered success envelope")
}
