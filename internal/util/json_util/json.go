// Package json_util provides JSON utilities including a custom RawMessage type.
package json_util

import (
	"errors"
)

// RawMessage is a custom JSON raw message type that marshals empty slices as "null".
type RawMessage []byte

// MarshalJSON customizes the JSON marshaling behavior for RawMessage.
// Empty RawMessage values are marshaled as "null" instead of "[]".
func (m RawMessage) MarshalJSON() ([]byte, error) {
	if len(m) == 0 {
		return []byte("null"), nil
	}
	return m, nil
}

// UnmarshalJSON sets *m to a copy of the JSON data.
func (m *RawMessage) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("json.RawMessage: UnmarshalJSON on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil
}
