// Package fakeserver serves embedded contracts and canned fixtures for acceptance.
package fakeserver

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/ContractorKeith/slowbooks-cli/testdata"
)

type Request struct {
	Method, URI string
	Header      http.Header
	Body        json.RawMessage
}
type Server struct {
	mu       sync.Mutex
	requests []Request
}

// New constructs a fake with an independent request recorder.
func New() *Server { return &Server{} }

// Requests returns a snapshot safe to inspect while the server is running.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Request, len(s.requests))
	for i, req := range s.requests {
		result[i] = req
		result[i].Header = req.Header.Clone()
		result[i].Body = append(json.RawMessage(nil), req.Body...)
	}
	return result
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	s.mu.Lock()
	s.requests = append(s.requests, Request{r.Method, r.URL.RequestURI(), r.Header.Clone(), body})
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		_, _ = w.Write([]byte(`{"detail":"Method not allowed"}`))
		return
	}
	var name string
	switch r.URL.Path {
	case "/openapi.json":
		name = "openapi-2.31.10.json"
	case "/api/system":
		name = "fake/system.json"
	default:
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"detail":"Not found"}`))
		return
	}
	data, err := testdata.Files.ReadFile(name)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	_, _ = w.Write(data)
}
