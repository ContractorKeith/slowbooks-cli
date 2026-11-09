package cli

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
)

func TestNamedWriteNetworkFailures(t *testing.T) {
	var attempts atomic.Int32
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != "POST" {
			return false
		}
		attempts.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
		} else {
			_ = conn.Close()
		}
		return true
	})
	for _, row := range commands.Writes() {
		before := attempts.Load()
		code, out, _ := invoke(t, "", writeArgs(row)...)
		hint := "sbp " + strings.Fields(row.Name)[0] + " list --limit 5"
		retryable := `"retryable":false`
		if strings.HasSuffix(row.Name, "record") {
			hint = "rerun with the same --key"
			retryable = `"retryable":true`
		}
		if code != 8 || attempts.Load() != before+1 || !strings.Contains(out, hint) || !strings.Contains(out, retryable) {
			t.Fatal(row.Name, code, out, attempts.Load()-before)
		}
	}
}
