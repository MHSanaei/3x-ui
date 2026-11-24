package json_util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrListTooLong is returned by DecodeCappedList for a list past its entry limit.
var ErrListTooLong = errors.New("list exceeds entry limit")

// DecodeCappedList decodes a JSON array entry by entry and stops at the first entry past
// maxEntries: each "{}" becomes a whole T, so a byte cap alone does not bound the result.
func DecodeCappedList[T any](raw []byte, maxEntries int) ([]T, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil {
		return nil, err
	} else if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return nil, fmt.Errorf("expected a JSON array, got %v", tok)
	}
	var out []T
	for dec.More() {
		if len(out) >= maxEntries {
			return nil, fmt.Errorf("%w (%d)", ErrListTooLong, maxEntries)
		}
		var item T
		if err := dec.Decode(&item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}
