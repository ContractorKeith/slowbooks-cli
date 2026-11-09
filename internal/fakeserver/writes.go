package fakeserver

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
)

type writeReplay struct{ body, result json.RawMessage }

// writeFixture models role refusals and payment replay without mutating read fixtures.
func (s *Server) writeFixture(w http.ResponseWriter, r *http.Request, body json.RawMessage, token string) bool {
	if r.Method != "POST" {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/"), "/")
	kind := parts[0]
	void := len(parts) == 3 && parts[2] == "void"
	switch kind {
	case "invoices", "payments", "bill-payments":
	case "bills", "expenses", "journal":
		if !void {
			return false
		}
	default:
		return false
	}
	if !void && len(parts) != 1 {
		return false
	}
	if token == "sbp_readonly" {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"detail":"This token is read-only"}`))
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	keyed := !void && (kind == "payments" || kind == "bill-payments") && r.Header.Get("Idempotency-Key") != ""
	key := r.Header.Get("X-SlowBooks-Company") + ":" + kind + ":" + r.Header.Get("Idempotency-Key")
	var compact bytes.Buffer
	_ = json.Compact(&compact, body)
	if keyed {
		if replay, ok := s.replays[key]; ok {
			if !bytes.Equal(replay.body, compact.Bytes()) {
				w.WriteHeader(409)
				_, _ = w.Write([]byte(`{"detail":"Payment key reused with a different body"}`))
			} else {
				_, _ = w.Write(replay.result)
			}
			return true
		}
	}
	raw, err := fs.ReadFile(s.fixtures, "fake/"+kind+".json")
	var fixtures []json.RawMessage
	if err != nil || json.Unmarshal(raw, &fixtures) != nil || len(fixtures) == 0 {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"detail":"Write fixture unavailable"}`))
		return true
	}
	result := fixtures[0]
	if void {
		result = json.RawMessage(`{"id":1,"status":"void"}`)
		if kind == "payments" || kind == "bill-payments" {
			result = json.RawMessage(`{"id":1,"is_voided":true}`)
		}
	}
	if keyed {
		if s.replays == nil {
			s.replays = map[string]writeReplay{}
		}
		s.replays[key] = writeReplay{append(json.RawMessage(nil), compact.Bytes()...), result}
	}
	_, _ = w.Write(result)
	return true
}
