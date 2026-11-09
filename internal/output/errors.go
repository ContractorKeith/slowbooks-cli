package output

import "errors"

// Error is the stable public error contract. Detail retains the server's JSON.
type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	Hint      string `json:"hint"`
	Detail    any    `json:"detail"`
}

func (e *Error) Error() string { return e.Message }

// AsError keeps typed errors and gives unexpected errors the generic code.
func AsError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: "error", Message: err.Error()}
}

// ExitCode is the single mapping used by every command.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	switch AsError(err).Code {
	case "usage":
		return 2
	case "auth":
		return 3
	case "refused":
		return 4
	case "not_found":
		return 5
	case "conflict":
		return 6
	case "invalid":
		return 7
	case "network", "server":
		return 8
	default:
		return 1
	}
}
