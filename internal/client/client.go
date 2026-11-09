// Package client owns all outbound HTTP and the server error mapping.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

type Options struct {
	URL, Token, Company, Version string
	Timeout                      time.Duration
}

type Client struct {
	base             *url.URL
	http             *http.Client
	options          Options
	maxResponseBytes int64
	keyedOperation   bool
}

// New enforces transport policy before credentials can be sent.
func New(ctx context.Context, options Options) (*Client, error) {
	return newClient(ctx, options, defaultLookup)
}

func newClient(ctx context.Context, options Options, lookup lookupFunc) (*Client, error) {
	if err := validateHeaders(options); err != nil {
		return nil, err
	}
	base, err := url.Parse(options.URL)
	if err != nil || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return nil, usage("server URL must contain only a scheme and host")
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, usage("server URL requires http or https")
	}
	if options.Timeout == 0 {
		options.Timeout = 120 * time.Second
	}
	if options.Timeout < time.Second || options.Timeout > 3600*time.Second {
		return nil, usage("timeout must be 1–3600 seconds")
	}
	lookupCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	transport, err := transportFor(lookupCtx, base, lookup)
	if err != nil {
		if lookupCtx.Err() != nil {
			return nil, &output.Error{Code: "network", Message: "host lookup failed or timed out", Hint: "check the server URL and connection"}
		}
		return nil, err
	}
	// A fresh connection per request prevents net/http from replaying a read on a
	// stale pooled connection outside our two-attempt budget. CLI calls favor a
	// predictable wire count over connection pooling.
	transport.DisableKeepAlives = true
	return &Client{base: base, options: options, maxResponseBytes: 64 << 20, http: &http.Client{
		Transport: transport, Timeout: options.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// Do retries reads once; writes are always a single attempt.
func (c *Client) Do(ctx context.Context, method, path string, body []byte, key string) ([]byte, error) {
	response, err := c.DoResponse(ctx, method, path, body, key)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

// Response retains the media type so callers can distinguish text from broken JSON.
type Response struct {
	Data        []byte
	ContentType string
}

func (c *Client) DoResponse(ctx context.Context, method, path string, body []byte, key string) (*Response, error) {
	if key != "" {
		if err := ValidateKey(key); err != nil {
			return nil, err
		}
	}
	method = strings.ToUpper(method)
	read := method == http.MethodGet || method == http.MethodHead
	for attempt := 0; ; attempt++ {
		data, err := c.attempt(ctx, method, path, body, key)
		if err == nil {
			return data, nil
		}
		e := output.AsError(err)
		if !read && key != "" && c.keyedOperation && (e.Code == "network" || e.Code == "server" && WriteReachedWire(err)) && e.Message != "response too large" {
			e.Retryable = true
			e.Hint = "rerun with the same --key"
		}
		if !read || attempt == 1 || !e.Retryable || ctx.Err() != nil {
			return nil, err
		}
	}
}

func (c *Client) attempt(ctx context.Context, method, path string, body []byte, key string) (*Response, error) {
	method = strings.ToUpper(method)
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return nil, usage("method must be GET, HEAD, POST, PUT, PATCH or DELETE")
	}
	target, err := requestURL(c.base, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, usage("invalid request method")
	}
	if c.options.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.options.Token)
	}
	if c.options.Company != "" {
		req.Header.Set("X-SlowBooks-Company", c.options.Company)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	req.Header.Set("User-Agent", "slowbooks/"+c.options.Version)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	read := method == http.MethodGet || method == http.MethodHead
	if !read {
		// Idempotency-Key normally enables Go transport replay. Remove GetBody and
		// keep even empty writes non-replayable, including on stale connections.
		req.GetBody = nil
		req.Body = io.NopCloser(bytes.NewReader(body))
		if len(body) == 0 {
			req.ContentLength = -1
		}
	}
	var sent atomic.Bool
	if !read {
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{WroteHeaders: func() { sent.Store(true) }}))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, requestNetworkError("request failed or timed out", read, sent.Load())
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
	if err != nil {
		return nil, requestNetworkError("could not read server response", read, sent.Load())
	}
	if int64(len(data)) > c.maxResponseBytes {
		err := networkError("response too large", read)
		err.Retryable = false
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &Response{Data: data, ContentType: resp.Header.Get("Content-Type")}, nil
	}
	e := responseError(resp.StatusCode, data, read)
	if !read && (resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504) {
		e.Hint = "check the books before running this write again"
		return nil, &writeAttemptError{e}
	}
	return nil, e
}

func networkError(message string, read bool) *output.Error {
	hint := "check the server URL and connection"
	if !read {
		hint = "check the books before running this write again"
	}
	return &output.Error{Code: "network", Message: message, Retryable: read, Hint: hint}
}

func usage(message string) *output.Error { return &output.Error{Code: "usage", Message: message} }

func requestURL(base *url.URL, path string) (*url.URL, error) {
	ref, err := url.Parse(path)
	if err != nil || ref.IsAbs() || ref.Host != "" || ref.Fragment != "" || ref.Opaque != "" {
		return nil, usage("only /api/ paths and /openapi.json are allowed")
	}
	if _, err := url.ParseQuery(ref.RawQuery); err != nil {
		return nil, usage("malformed query string")
	}
	// Reject ambiguous encodings and traversal before a proxy can normalize them.
	if strings.ContainsAny(ref.Path, "\\%") || strings.Contains(ref.Path, "//") {
		return nil, usage("ambiguous request path")
	}
	for _, part := range strings.Split(ref.Path, "/") {
		if part == "." || part == ".." {
			return nil, usage("path traversal is not allowed")
		}
	}
	if !strings.HasPrefix(ref.Path, "/api/") && ref.Path != "/openapi.json" {
		return nil, usage("only /api/ paths and /openapi.json are allowed")
	}
	return base.ResolveReference(ref), nil
}

func responseError(status int, data []byte, read bool) *output.Error {
	e := &output.Error{Code: "error", Message: http.StatusText(status)}
	switch {
	case status >= 300 && status < 400:
		e.Code = "usage"
		e.Message = "server redirect refused"
	case status == 401:
		e.Code = "auth"
	case status == 403:
		e.Code = "refused"
	case status == 404:
		e.Code = "not_found"
	case status == 409:
		e.Code = "conflict"
	case status == 400 || status == 422:
		e.Code = "invalid"
	case status >= 500:
		e.Code = "server"
		e.Retryable = read && (status == 502 || status == 503 || status == 504)
	}
	var payload struct {
		Detail json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Detail != nil {
		e.Detail = payload.Detail
		if message := detailMessage(payload.Detail); message != "" && !(status >= 300 && status < 400) {
			e.Message = message
		}
	}
	return e
}

// WithoutCompany shares the transport but probes token-wide permissions.
func (c *Client) WithoutCompany() *Client {
	clone := *c
	clone.options.Company = ""
	return &clone
}
