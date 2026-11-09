package output

import (
	"context"
	"fmt"
	"os"
	"runtime/metrics"
	"time"
)

const jqHeapLimit = 512 << 20

// consumeJQ is the only writer. Cancellation gives the evaluator a short grace
// period; a stuck evaluator cannot retain an unbounded heap in a live process.
func (w *Writer) consumeJQ(ctx context.Context, cancel context.CancelFunc, results <-chan jqResult, baseline uint64) error {
	limit := w.jqMemoryLimit
	if limit == 0 {
		limit = jqHeapLimit
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	samples := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	var memoryErr *Error
	var grace *time.Timer
	var expired <-chan time.Time
	ctxDone := ctx.Done()
	defer func() {
		if grace != nil {
			grace.Stop()
		}
	}()
	for {
		select {
		case result, ok := <-results:
			if !ok {
				if memoryErr != nil {
					return memoryErr
				}
				if ctx.Err() != nil {
					return jqBound("context cancelled or time limit exceeded")
				}
				return nil
			}
			if memoryErr != nil {
				continue
			}
			if ctx.Err() != nil {
				return jqBound("context cancelled or time limit exceeded")
			}
			if result.err != nil {
				return result.err
			}
			if _, err := w.Out.Write(result.data); err != nil {
				return err
			}
		case <-ticker.C:
			if memoryErr != nil {
				continue
			}
			metrics.Read(samples)
			heap := samples[0].Value.Uint64()
			if heap > baseline && heap-baseline > limit {
				memoryErr = &Error{Code: "usage", Message: fmt.Sprintf("--jq used more than %d MiB", limit>>20)}
				cancel()
				ctxDone = nil
				grace = time.NewTimer(250 * time.Millisecond)
				expired = grace.C
			}
		case <-ctxDone:
			return jqBound("context cancelled or time limit exceeded")
		case <-expired:
			// A known successful write must remain visible even if its filter is stuck.
			if w.writeResult != nil {
				if err := w.recoverWriteFilter(memoryErr); err != nil {
					os.Exit(1)
				}
				os.Exit(0)
			}
			// Only this goroutine writes output, including the final error envelope.
			w.JSON = true
			_ = w.Failure(memoryErr)
			os.Exit(2)
		}
	}
}

// The input already exists when this baseline is sampled; only growth during
// evaluation counts against the jq budget.
func jqHeapUsage() uint64 {
	samples := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(samples)
	return samples[0].Value.Uint64()
}
