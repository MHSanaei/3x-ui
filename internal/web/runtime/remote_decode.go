package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Per-endpoint caps on node-reported lists: each "{}" decodes into a whole struct, so the
// byte cap alone still lets a node's answer expand into gigabytes.
const (
	maxNodeDescendants = 1024
	maxNodeInbounds    = 65536
	maxNodeHostGroups  = 65536
	maxNodeClientIPs   = 1 << 20
	// maxNodeMapValues bounds every value of the GUID-keyed trees and maps a node reports.
	maxNodeMapValues = 1 << 20
)

var errRemoteListTooLong = errors.New("remote list exceeds entry limit")

// decodeCappedList decodes a JSON array entry by entry and stops at the first entry
// past maxEntries; null decodes to a nil slice.
func decodeCappedList[T any](raw json.RawMessage, maxEntries int) ([]T, error) {
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
			return nil, fmt.Errorf("%w (%d)", errRemoteListTooLong, maxEntries)
		}
		var item T
		if err := dec.Decode(&item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// decodeCappedValues unmarshals raw into v after checking it holds at most maxValues
// JSON values in all, nested ones included.
func decodeCappedValues(raw json.RawMessage, v any, maxValues int) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	for count := 0; ; count++ {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if delim, ok := tok.(json.Delim); ok && (delim == ']' || delim == '}') {
			continue
		}
		if count > maxValues {
			return fmt.Errorf("%w (%d values)", errRemoteListTooLong, maxValues)
		}
	}
	return json.Unmarshal(raw, v)
}

// truncateNodeMessage bounds a node-supplied message the master turns into an error,
// which is logged and stored as the node's lastError.
func truncateNodeMessage(msg string) string {
	if len(msg) <= errBodyDiagBytes {
		return msg
	}
	cut := errBodyDiagBytes
	for cut > 0 && !utf8.RuneStart(msg[cut]) {
		cut--
	}
	return msg[:cut] + "... (truncated)"
}
