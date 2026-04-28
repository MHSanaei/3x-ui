package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnwrapXrayTemplateConfig(t *testing.T) {
	real := `{"log":{},"inbounds":[],"outbounds":[],"routing":{}}`

	t.Run("passes through a clean config", func(t *testing.T) {
		if got := UnwrapXrayTemplateConfig(real); got != real {
			t.Fatalf("clean config was modified: %s", got)
		}
	})

	t.Run("passes through invalid JSON unchanged", func(t *testing.T) {
		in := "not json at all"
		if got := UnwrapXrayTemplateConfig(in); got != in {
			t.Fatalf("invalid input was modified: %s", got)
		}
	})

	t.Run("unwraps one layer of response-shaped wrapper", func(t *testing.T) {
		wrapper := `{"inboundTags":["tag"],"outboundTestUrl":"x","xraySetting":` + real + `}`
		got := UnwrapXrayTemplateConfig(wrapper)
		if !equalJSON(t, got, real) {
			t.Fatalf("want %s, got %s", real, got)
		}
	})

	t.Run("unwraps multiple stacked layers", func(t *testing.T) {
		lvl1 := `{"xraySetting":` + real + `}`
		lvl2 := `{"xraySetting":` + lvl1 + `}`
		lvl3 := `{"xraySetting":` + lvl2 + `}`
		got := UnwrapXrayTemplateConfig(lvl3)
		if !equalJSON(t, got, real) {
			t.Fatalf("want %s, got %s", real, got)
		}
	})

	t.Run("handles an xraySetting stored as a JSON-encoded string", func(t *testing.T) {
		encoded, _ := json.Marshal(real) // becomes a quoted string
		wrapper := `{"xraySetting":` + string(encoded) + `}`
		got := UnwrapXrayTemplateConfig(wrapper)
		if !equalJSON(t, got, real) {
			t.Fatalf("want %s, got %s", real, got)
		}
	})

	t.Run("does not unwrap when top level already has real xray keys", func(t *testing.T) {
		// Pathological but defensible: if a user's actual config somehow
		// has both the real keys and an unrelated `xraySetting` key, we
		// must not strip it.
		in := `{"inbounds":[],"xraySetting":{"some":"thing"}}`
		got := UnwrapXrayTemplateConfig(in)
		if got != in {
			t.Fatalf("should have left real config alone, got %s", got)
		}
	})

	t.Run("stops at a reasonable depth", func(t *testing.T) {
		// Build a deeper-than-maxDepth chain that ends at something
		// non-wrapped, and confirm we end up at some valid JSON (we
		// don't loop forever and we don't blow the stack).
		s := real
		for i := 0; i < 16; i++ {
			s = `{"xraySetting":` + s + `}`
		}
		got := UnwrapXrayTemplateConfig(s)
		if !strings.Contains(got, `"inbounds"`) && !strings.Contains(got, `"xraySetting"`) {
			t.Fatalf("unexpected tail: %s", got)
		}
	})
}

func equalJSON(t *testing.T, a, b string) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal([]byte(a), &va); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &vb); err != nil {
		return false
	}
	ja, _ := json.Marshal(va)
	jb, _ := json.Marshal(vb)
	return string(ja) == string(jb)
}

func TestStripLegacyReverse_RemovesPortalsAndBridges(t *testing.T) {
	// #4115: this is the exact shape the panel UI used to write and
	// xray-core v26+ now refuses to parse.
	in := `{
		"inbounds":[],
		"reverse":{
			"portals":[{"tag":"Portal1","domain":"reverse.xui1"}],
			"bridges":[{"tag":"Bridge1","domain":"reverse.xui1"}]
		}
	}`
	out, removed, err := StripLegacyReverse(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !removed {
		t.Fatalf("removed flag should be true when legacy reverse block is present")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("output is not valid json: %v\n%s", err, out)
	}
	if _, still := cfg["reverse"]; still {
		t.Fatalf("reverse block should have been removed, got: %s", out)
	}
	if _, ok := cfg["inbounds"]; !ok {
		t.Fatalf("unrelated fields should be preserved, got: %s", out)
	}
}

func TestStripLegacyReverse_NoOpWhenNoReverseBlock(t *testing.T) {
	// Don't touch configs that never had legacy reverse in the first
	// place. Saving stays a no-op so the diff in the panel UI stays
	// quiet.
	in := `{"inbounds":[],"outbounds":[],"routing":{}}`
	out, removed, err := StripLegacyReverse(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if removed {
		t.Fatalf("removed flag should be false when there's no reverse block")
	}
	if out != in {
		t.Fatalf("expected unchanged input, got: %s", out)
	}
}

func TestStripLegacyReverse_LeavesNonLegacyReverseAlone(t *testing.T) {
	// The new VLESS Reverse Proxy lives as a `reverse` field on a VLESS
	// client (inside inbound.settings.clients[].reverse), NOT at the
	// top level. But just in case some future xray version puts
	// something else under top-level `reverse` that's not the legacy
	// shape, leave it alone if neither `portals` nor `bridges` are
	// present.
	in := `{"reverse":{"someFutureField":42}}`
	out, removed, err := StripLegacyReverse(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if removed {
		t.Fatalf("removed flag should be false when reverse has no portals/bridges")
	}
	if out != in {
		t.Fatalf("expected unchanged input, got: %s", out)
	}
}

func TestStripLegacyReverse_DoesNotTouchNestedReverseFields(t *testing.T) {
	// VLESS Reverse Proxy puts a `reverse` field inside an inbound
	// client. Make sure we only target the TOP-LEVEL key, not anything
	// nested.
	in := `{"inbounds":[{"settings":{"clients":[{"id":"abc","reverse":{"tag":"r-out"}}]}}]}`
	out, removed, err := StripLegacyReverse(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if removed {
		t.Fatalf("nested client.reverse must not be touched, removed should be false")
	}
	if out != in {
		t.Fatalf("nested client.reverse must not be touched\nin:  %s\nout: %s", in, out)
	}
}

func TestStripLegacyReverse_OnlyPortals(t *testing.T) {
	// Some configs have only `portals` (or only `bridges`). Either
	// alone is enough to trigger removal.
	in := `{"reverse":{"portals":[{"tag":"P","domain":"r.xui"}]}}`
	out, removed, err := StripLegacyReverse(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !removed {
		t.Fatalf("portals-only reverse should still be removed")
	}
	var cfg map[string]any
	json.Unmarshal([]byte(out), &cfg)
	if _, still := cfg["reverse"]; still {
		t.Fatalf("reverse should be gone, got: %s", out)
	}
}

func TestStripLegacyReverse_InvalidJsonReturnsError(t *testing.T) {
	// SaveXraySetting calls CheckXrayConfig after this helper, but we
	// want the helper itself to be defensive — return raw input plus
	// an error if the JSON is unparseable, so the caller can decide
	// whether to skip or block.
	in := "not json"
	out, removed, err := StripLegacyReverse(in)
	if err == nil {
		t.Fatalf("expected error for invalid json, got none")
	}
	if removed {
		t.Fatalf("nothing should be removed on parse error")
	}
	if out != in {
		t.Fatalf("expected raw passthrough on error, got %q", out)
	}
}
