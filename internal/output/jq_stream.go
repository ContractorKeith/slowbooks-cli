package output

import (
	"bytes"
	"context"
	"fmt"

	"github.com/itchyny/gojq"
)

type jqResult struct {
	data []byte
	err  error
}

// Evaluation and encoding run away from the writer so heap checks remain live
// even while Next or JSON encoding is allocating an intermediate value.
func streamJQ(ctx context.Context, code *gojq.Code, value any, results chan<- jqResult) {
	defer close(results)
	send := func(result jqResult) bool {
		select {
		case results <- result:
			return true
		case <-ctx.Done():
			return false
		}
	}
	iter := code.RunWithContext(ctx, value)
	remaining := 64 << 20
	for count := 0; ctx.Err() == nil; count++ {
		value, ok := iter.Next()
		if !ok || ctx.Err() != nil {
			return
		}
		if count >= 100000 {
			send(jqResult{err: jqBound("output exceeds 100000 results")})
			return
		}
		if err, ok := value.(error); ok {
			send(jqResult{err: &Error{Code: "usage", Message: fmt.Sprintf("--jq: %v", err)}})
			return
		}
		var buf bytes.Buffer
		if err := jsonEncoder(&buf).Encode(value); err != nil {
			send(jqResult{err: err})
			return
		}
		encoded := buf.Bytes()
		if len(encoded) > remaining {
			send(jqResult{err: jqBound("output exceeds 64 MiB")})
			return
		}
		remaining -= len(encoded)
		if !send(jqResult{data: encoded}) {
			return
		}
	}
}
