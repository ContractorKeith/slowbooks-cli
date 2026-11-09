package client

import (
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
		body, err = io.ReadAll(input)
	case strings.HasPrefix(data, "@"):
		body, err = os.ReadFile(strings.TrimPrefix(data, "@"))
	default:
		body = []byte(data)
	}
	if err != nil {
		return nil, &output.Error{Code: "usage", Message: "could not read --data"}
	}
	if !json.Valid(body) {
		return nil, &output.Error{Code: "usage", Message: "--data must be valid JSON"}
	}
	return body, nil
}
