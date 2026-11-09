package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/url"
	"strconv"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/catalog"
	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

// callResult keeps paging and exact JSON decoding on the generic request path.
func (s *session) callResult(cmd *cobra.Command, cl *client.Client, method string, target *url.URL, body []byte, opts callOptions, op catalog.Operation) (any, bool, error) {
	query := target.Query()
	hasLimit, pageParam, pageSize, maximum := paging(op)
	if !opts.list {
		hasLimit = false
		pageParam = ""
	}
	if hasLimit {
		size := pageSize
		if !opts.all {
			size = opts.limit + 1
			if maximum > 0 {
				size = min(size, maximum)
			}
		}
		query.Set("limit", strconv.Itoa(size))
		pageSize = size
	}
	offset := 0
	combined := []any{}
	for {
		if opts.list && pageParam != "" && (opts.all || hasLimit) {
			query.Set(pageParam, strconv.Itoa(offset))
		}
		target.RawQuery = query.Encode()
		response, err := cl.DoResponse(cmd.Context(), method, target.String(), body, opts.key)
		if err != nil {
			return nil, false, err
		}
		result, err := decodeResult(response.Data, response.ContentType, method, op.ResponseMedia)
		if err != nil {
			return nil, false, err
		}
		rows, isArray := result.([]any)
		if !opts.list || !isArray {
			return result, false, nil
		}
		if !opts.all && (!hasLimit || pageParam == "") {
			return result, hasLimit && len(rows) > opts.limit, nil
		}
		if pageParam == "" {
			if hasLimit && len(rows) >= pageSize {
				if err := s.warn(fmt.Sprintf("%s returned its limit of %d rows without paging; results may be incomplete", op.Path, pageSize)); err != nil {
					return nil, false, err
				}
			}
			return result, false, nil
		}
		combined = append(combined, rows...)
		if !opts.all && len(combined) > opts.limit {
			return combined, true, nil
		}
		if len(rows) == 0 || (hasLimit && len(rows) < pageSize) {
			return combined, false, nil
		}
		offset += len(rows)
	}
}

// paging uses a declared maximum, otherwise the declared default, then 500.
func paging(op catalog.Operation) (hasLimit bool, pageParam string, size, maximum int) {
	size = 500
	for _, p := range op.Params {
		if p.In != "query" {
			continue
		}
		if p.Name == "skip" || p.Name == "offset" {
			pageParam = p.Name
		}
		if p.Name == "limit" {
			hasLimit = true
			schema, _ := p.Schema.(map[string]any)
			for _, key := range []string{"maximum", "default"} {
				if n, ok := schema[key].(float64); ok && n >= 1 && n < float64(int(^uint(0)>>1)) {
					size = int(n)
					if key == "maximum" {
						maximum = size
					}
					break
				}
			}
		}
	}
	return
}

func decodeResult(data []byte, contentType, method string, declared []string) (any, error) {
	if len(data) == 0 || method == "HEAD" {
		return nil, nil
	}
	media, _, _ := mime.ParseMediaType(contentType)
	if method != "GET" && media != "application/json" && !strings.HasSuffix(media, "+json") {
		if requiresJSON(declared, media) {
			return nil, uncertainJSONWrite()
		}
		return string(data), nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var result any
	if err := decoder.Decode(&result); err != nil {
		if method != "GET" && method != "HEAD" && requiresJSON(declared, "") {
			return nil, uncertainJSONWrite()
		}
		return nil, &output.Error{Code: "error", Message: "invalid JSON from server"}
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if method != "GET" && method != "HEAD" && requiresJSON(declared, "") {
			return nil, uncertainJSONWrite()
		}
		return nil, &output.Error{Code: "error", Message: "invalid JSON from server"}
	}
	return result, nil
}

// Explicitly declared non-JSON responses remain valid, including mixed media.
func requiresJSON(declared []string, actual string) bool {
	jsonDeclared := false
	for _, media := range declared {
		if media == actual {
			return false
		}
		if media == "application/json" || strings.HasSuffix(media, "+json") {
			jsonDeclared = true
		}
	}
	return jsonDeclared
}

func uncertainJSONWrite() *output.Error {
	return &output.Error{Code: "error", Message: "the server answered with a page, not JSON; the write may have happened", Hint: "check the books before running this write again"}
}
