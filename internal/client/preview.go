package client

import "strings"

type Preview struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

// Preview validates the target without sending a request or exposing credentials.
func (c *Client) Preview(method, path string, body any, key string) (Preview, error) {
	if key != "" {
		if err := ValidateKey(key); err != nil {
			return Preview{}, err
		}
	}
	method = strings.ToUpper(method)
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
	default:
		return Preview{}, usage("method must be GET, HEAD, POST, PUT, PATCH or DELETE")
	}
	target, err := requestURL(c.base, path)
	if err != nil {
		return Preview{}, err
	}
	headers := map[string]string{"User-Agent": "sbp/" + c.options.Version}
	if c.options.Token != "" {
		headers["Authorization"] = "Bearer <redacted>"
	}
	if c.options.Company != "" {
		headers["X-SlowBooks-Company"] = c.options.Company
	}
	if key != "" {
		headers["Idempotency-Key"] = key
	}
	if body != nil {
		headers["Content-Type"] = "application/json"
	}
	return Preview{method, target.String(), headers, body}, nil
}
