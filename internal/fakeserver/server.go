// Package fakeserver serves embedded contracts and canned fixtures for acceptance.
package fakeserver

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/ContractorKeith/slowbooks-cli/testdata"
)

type Request struct {
	Method, URI string
	Header      http.Header
	Body        json.RawMessage
}
type Server struct {
	mu        sync.Mutex
	fixtures  fs.FS
	requests  []Request
	customers []json.RawMessage
	replays   map[string]writeReplay
}

// New constructs a fake with an independent request recorder.
func New() *Server { return &Server{fixtures: testdata.Files} }

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
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token != "" && token != "sbp_no_companies" && token != "sbp_test" && token != "sbp_bookkeeper" && token != "sbp_accountant" && token != "sbp_readonly" {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"detail":"Invalid token"}`))
		return
	}
	if r.URL.Path == "/api/customers" {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.Method == "POST" {
			s.customers = append(s.customers, body)
			_, _ = w.Write(body)
			return
		}
		if r.Method == "GET" {
			customers := append(customerFixtures(), s.customers...)
			if customers == nil {
				customers = []json.RawMessage{}
			}
			_ = json.NewEncoder(w).Encode(customers)
			return
		}
	}
	if s.writeFixture(w, r, body, token) {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		_, _ = w.Write([]byte(`{"detail":"Method not allowed"}`))
		return
	}
	if (r.URL.Path == "/api/companies" && token == "sbp_no_companies") || (r.URL.Path == "/api/audit" && token == "sbp_readonly") || (r.URL.Path == "/api/employees" && (token == "sbp_readonly" || token == "sbp_accountant")) {
		w.WriteHeader(403)
		data, _ := testdata.Files.ReadFile("fake/forbidden.json")
		_, _ = w.Write(data)
		return
	}
	if s.readFixture(w, r) || serveDownload(w, r) {
		return
	}
	var name string
	switch r.URL.Path {
	case "/openapi.json":
		name = "openapi-2.31.10.json"
	case "/api/audit":
		name = "fake/audit.json"
	case "/api/employees":
		name = "fake/employees.json"
	case "/api/companies":
		name = "fake/companies.json"
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
