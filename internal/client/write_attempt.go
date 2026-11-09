package client

import "errors"

type writeAttemptError struct{ error }

func (e *writeAttemptError) Unwrap() error { return e.error }

// WriteReachedWire distinguishes uncertain writes from lookup, dial and discovery failures.
func WriteReachedWire(err error) bool {
	var attempted *writeAttemptError
	return errors.As(err, &attempted)
}

func requestNetworkError(message string, read, sent bool) error {
	e := networkError(message, read)
	if !read && sent {
		return &writeAttemptError{e}
	}
	if !read {
		e.Hint = "check the server URL and connection"
	}
	return e
}
