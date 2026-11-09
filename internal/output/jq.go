package output

import (
	"encoding/json"
	"fmt"

	"github.com/itchyny/gojq"
)

// SetJQ compiles before any command side effects, including an explicitly empty filter.
func (w *Writer) SetJQ(expr string, enabled bool) error {
	w.jq = nil
	if !enabled {
		return nil
	}
	query, err := gojq.Parse(expr)
	if err != nil {
		return &Error{Code: "usage", Message: "--jq: " + err.Error()}
	}
	w.jq, err = gojq.Compile(query)
	if err != nil {
		return &Error{Code: "usage", Message: "--jq: " + err.Error()}
	}
	return nil
}

func (w *Writer) filter(value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var input any
	if err = json.Unmarshal(raw, &input); err != nil {
		return err
	}
	iter := w.jq.Run(input)
	encoder := json.NewEncoder(w.Out)
	for {
		value, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, ok := value.(error); ok {
			return &Error{Code: "usage", Message: fmt.Sprintf("--jq: %v", err)}
		}
		if err = encoder.Encode(value); err != nil {
			return err
		}
	}
}
