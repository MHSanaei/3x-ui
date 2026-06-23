package xray

import "testing"

func TestStatEmailRoundTrip(t *testing.T) {
	cases := []struct {
		id    int
		email string
	}{
		{1, "alice"},
		{42, "user@example.com"},
		{7, "weird::email::value"}, // an email containing the separator still round-trips
		{1000000, ""},
	}
	for _, c := range cases {
		enc := EncodeStatEmail(c.id, c.email)
		id, email, ok := DecodeStatEmail(enc)
		if !ok || id != c.id || email != c.email {
			t.Errorf("round trip %q -> %q: got (id=%d, email=%q, ok=%v), want (%d, %q, true)",
				c.email, enc, id, email, ok, c.id, c.email)
		}
	}
}

func TestDecodeStatEmailLegacy(t *testing.T) {
	// A legacy (un-encoded) email has no numeric "<id>::" prefix and must pass
	// through unchanged with ok=false, so a mixed-state upgrade keeps working.
	for _, raw := range []string{"alice", "user@example.com", "noprefix", "::leadingsep", "x::y"} {
		id, email, ok := DecodeStatEmail(raw)
		if ok || id != 0 || email != raw {
			t.Errorf("legacy %q: got (id=%d, email=%q, ok=%v), want (0, %q, false)", raw, id, email, ok, raw)
		}
	}
}
