package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestR2SuccessfulWriteSurvivesJQRuntimeFailure(t *testing.T) {
	writes := 0
	reviewServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != "POST" {
			return false
		}
		writes++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42}`))
		return true
	})
	for _, args := range [][]string{{"api", "POST", "/api/customers", "--data", "{}"}, {"invoices", "create", "--customer", "1", "--date", "2026-11-09"}, {"payments", "record", "--key", "jq-test", "--customer", "1", "--amount", "5", "--date", "2026-11-09"}} {
		for _, expr := range []string{`error("failed")`, `.data.id,error("after a result")`} {
			for _, quiet := range []bool{false, true} {
				call := append(append([]string{}, args...), "--jq", expr)
				if quiet {
					call = append(call, "--quiet")
				}
				before := writes
				code, out, stderr := invoke(t, "", call...)
				var envelope struct {
					OK   bool
					Data struct{ ID int }
				}
				dec := json.NewDecoder(strings.NewReader(out))
				err := dec.Decode(&envelope)
				if code != 0 || err != nil || !envelope.OK || envelope.Data.ID != 42 || dec.Decode(new(any)) != io.EOF || writes != before+1 || !strings.Contains(stderr, "--jq failed after successful write") {
					t.Errorf("%v: %d %s %s", call, code, out, stderr)
				}
			}
		}
	}
}
