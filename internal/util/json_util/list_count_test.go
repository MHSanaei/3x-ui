package json_util

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func FuzzCountCappedList(f *testing.F) {
	for _, raw := range []string{`[]`, `[{},[],null,"comma, bracket ] quote \""]`, `[1,2]`, `[1`, `[1] [2]`, `{}`, `null`} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			return
		}
		var rows []json.RawMessage
		err := json.Unmarshal([]byte(raw), &rows)
		if len(bytes.Trim([]byte(raw), " \t\r\n")) == 0 {
			err = nil
		}
		count, gotErr := CountCappedList([]byte(raw), 16)
		if err != nil {
			if gotErr == nil {
				t.Fatalf("accepted invalid array %q", raw)
			}
			return
		}
		if len(rows) > 16 {
			if !errors.Is(gotErr, ErrListTooLong) {
				t.Fatalf("count=%d error=%v, want list cap", count, gotErr)
			}
			return
		}
		if gotErr != nil || count != len(rows) {
			t.Fatalf("%q: count=%d error=%v want %d", raw, count, gotErr, len(rows))
		}
	})
}
