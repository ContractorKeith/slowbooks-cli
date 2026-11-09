package output

// ExitStatus describes one public exit-code mapping.
type ExitStatus struct {
	Exit int    `json:"exit"`
	Code string `json:"code"`
}

// ExitCodes uses the same mapping as command failures.
func ExitCodes() []ExitStatus {
	result := []ExitStatus{{Exit: ExitCode(nil), Code: "success"}}
	for _, code := range []string{"error", "usage", "auth", "refused", "not_found", "conflict", "invalid", "network", "server"} {
		result = append(result, ExitStatus{Exit: ExitCode(&Error{Code: code}), Code: code})
	}
	return result
}
