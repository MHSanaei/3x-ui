package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
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

var errRemoteListTooLong = json_util.ErrListTooLong

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
