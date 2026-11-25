package json_util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrListTooLong is returned by DecodeCappedList for a list past its entry limit.
var ErrListTooLong = errors.New("list exceeds entry limit")

// CountCappedList validates JSON, then counts top-level entries without allocating typed rows.
func CountCappedList(raw []byte, maxEntries int) (int, error) {
	raw = bytes.Trim(raw, " \t\r\n")
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, nil
	}
	if !json.Valid(raw) {
		return 0, fmt.Errorf("invalid JSON array")
	}
	if raw[0] != '[' {
		return 0, fmt.Errorf("expected a JSON array")
	}
	count, depth := 0, 0
	quoted, escaped, wantValue := false, false, true
	for _, c := range raw[1 : len(raw)-1] {
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if depth == 0 {
			if c == ',' {
				wantValue = true
				continue
			}
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
				continue
			}
			if wantValue {
				count++
				if count > maxEntries {
					return 0, fmt.Errorf("%w (%d)", ErrListTooLong, maxEntries)
				}
				wantValue = false
			}
		}
		switch c {
		case '"':
			quoted = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		}
	}
	return count, nil
}

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
