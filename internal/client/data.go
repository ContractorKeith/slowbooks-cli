package client

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

// ReadData loads the supported JSON body sources without leaking input on errors.
func ReadData(input io.Reader, data string) ([]byte, error) {
	var body []byte
	var err error
	switch {
	case data == "-":
		body, err = readBody(input)
	case strings.HasPrefix(data, "@"):
		var f *os.File
		f, err = os.Open(strings.TrimPrefix(data, "@"))
		if err == nil {
			defer f.Close()
			body, err = readBody(f)
		}
	default:
		body = []byte(data)
	}
	if err != nil {
		return nil, &output.Error{Code: "usage", Message: "could not read --data " + dataSource(data) + ": " + err.Error()}
	}
	if len(body) > maxRequestBytes {
		return nil, usage("--data exceeds 64 MiB")
	}
	if bytes.HasPrefix(body, []byte{0xff, 0xfe}) || bytes.HasPrefix(body, []byte{0xfe, 0xff}) {
		return nil, usage("UTF-16 is not supported; save as UTF-8")
	}
	body = bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf})
	if !json.Valid(body) {
		return nil, &output.Error{Code: "usage", Message: "--data must be valid JSON"}
	}
	return body, nil
}

const maxRequestBytes = 64 << 20

func readBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxRequestBytes+1))
	if err == nil && len(body) > maxRequestBytes {
		return nil, usage("--data exceeds 64 MiB")
	}
	return body, err
}
func dataSource(data string) string {
	if strings.HasPrefix(data, "@") {
		return data
	}
	return "stdin"
}
