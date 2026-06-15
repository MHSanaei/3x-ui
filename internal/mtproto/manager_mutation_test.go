package mtproto

import (
	"testing"
)

// TestParseMetricLineBraceBoundary pins the contract of the brace-position
// guard in parseMetricLine (manager.go:425 -> `if end < brace`).
//
// Once a '{' is found at index `brace`, the matching '}' must appear AFTER it.
// A '}' that precedes the '{', or a '{' with no closing '}' at all
// (strings.IndexByte returns -1, which is < brace), is a malformed line and
// must yield an error rather than slicing past the brace.
func TestParseMetricLineBraceBoundary(t *testing.T) {
	t.Run("closing brace before opening brace is malformed", func(t *testing.T) {
		// '}' at index 8 comes before '{' at index 16: end < brace must hold,
		// so this is rejected. Mutating `<` to `>`/`>=` would accept it.
		_, _, _, err := parseMetricLine(`mtg_x_a}_b{direction="x"} 5`)
		if err == nil {
			t.Fatal("expected error for '}' appearing before '{'")
		}
	})

	t.Run("opening brace with no closing brace is malformed", func(t *testing.T) {
		// No '}' at all -> end == -1, which is < brace. Must error.
		// If the guard were dropped/inverted the code would slice line[brace+1:-1]
		// and panic; asserting a clean error keeps that contract.
		_, _, _, err := parseMetricLine(`mtg_traffic{direction="x" 5`)
		if err == nil {
			t.Fatal("expected error for '{' without a closing '}'")
		}
	})

	t.Run("well-formed braces are accepted", func(t *testing.T) {
		// '{' at index 11, '}' at index 25: end > brace, so the guard must NOT
		// fire and parsing must succeed. Guards against a mutant that always errors.
		name, labels, val, err := parseMetricLine(`mtg_traffic{direction="up"} 42`)
		if err != nil {
			t.Fatalf("well-formed line should parse: %v", err)
		}
		if name != "mtg_traffic" {
			t.Fatalf("name=%q", name)
		}
		if labels["direction"] != "up" {
			t.Fatalf("labels=%v", labels)
		}
		if val != 42 {
			t.Fatalf("val=%v", val)
		}
	})
}

// TestParseMetricLineLabelEqualsBoundary pins the contract of the '=' guard in
// the per-label loop (manager.go:430 -> `if eq < 0`).
//
//   - eq < 0  (no '=' in the segment): the segment is skipped, no label added.
//   - eq == 0 (segment begins with '='): the key is empty but the pair is STILL
//     parsed, producing labels[""] = value. The boundary is `< 0`, not `<= 0`.
func TestParseMetricLineLabelEqualsBoundary(t *testing.T) {
	t.Run("label segment without '=' is skipped, not fatal", func(t *testing.T) {
		// "novalue" has no '=' (eq == -1) and must be skipped. A real key=val
		// segment in the same line must still be parsed. Mutating `< 0` to `> 0`
		// would take kv[:eq] with eq=-1 and panic; mutating away the skip would
		// also corrupt parsing.
		name, labels, val, err := parseMetricLine(`mtg_traffic{novalue,direction="down"} 9`)
		if err != nil {
			t.Fatalf("line with a value-less label should still parse: %v", err)
		}
		if name != "mtg_traffic" {
			t.Fatalf("name=%q", name)
		}
		if _, present := labels["novalue"]; present {
			t.Fatalf("value-less segment must not create a label: %v", labels)
		}
		if labels["direction"] != "down" {
			t.Fatalf("real label must still be parsed: %v", labels)
		}
		if val != 9 {
			t.Fatalf("val=%v", val)
		}
	})

	t.Run("label segment beginning with '=' is parsed as empty key", func(t *testing.T) {
		// "=onlyvalue": eq == 0. Since the guard is `< 0`, this is NOT skipped:
		// it yields labels[""] = "onlyvalue". A mutant changing `< 0` to `<= 0`
		// would skip it, losing the empty-key entry.
		_, labels, _, err := parseMetricLine(`mtg_traffic{=onlyvalue} 1`)
		if err != nil {
			t.Fatalf("segment with empty key should still parse: %v", err)
		}
		v, present := labels[""]
		if !present {
			t.Fatalf("eq==0 segment must produce an empty-key label: %v", labels)
		}
		if v != "onlyvalue" {
			t.Fatalf("empty-key label value=%q", v)
		}
	})
}
