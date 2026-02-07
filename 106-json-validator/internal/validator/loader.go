package validator

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ParseJSON decodes data with UseNumber, so numbers come back as json.Number
// and keep their full precision.
func ParseJSON(data []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("malformed JSON: %w", err)
	}

	// Decode stops after the first value; don't let "{} garbage" through.
	if dec.More() {
		return nil, fmt.Errorf("malformed JSON: unexpected trailing content")
	}
	return v, nil
}
