package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// xray-core v26.6.22 (#6258) renamed the XHTTP session keys with no fallback.
// The lift must rewrite stored configs at config-generation time so pre-upgrade
// inbounds/outbounds keep working without a manual re-save.
func TestLiftXhttpSessionIDKeys(t *testing.T) {
	t.Run("lifts legacy keys and drops them", func(t *testing.T) {
		stream := map[string]any{
			"xhttpSettings": map[string]any{
				"sessionPlacement": "cookie",
				"sessionKey":       "x_session",
			},
		}
		if !liftXhttpSessionIDKeys(stream) {
			t.Fatal("expected changed=true")
		}
		xhttp := stream["xhttpSettings"].(map[string]any)
		if xhttp["sessionIDPlacement"] != "cookie" || xhttp["sessionIDKey"] != "x_session" {
			t.Fatalf("renamed keys missing: %#v", xhttp)
		}
		if _, ok := xhttp["sessionPlacement"]; ok {
			t.Fatal("legacy sessionPlacement still present")
		}
		if _, ok := xhttp["sessionKey"]; ok {
			t.Fatal("legacy sessionKey still present")
		}
	})

	t.Run("keeps an explicit new key over the legacy one", func(t *testing.T) {
		stream := map[string]any{
			"xhttpSettings": map[string]any{
				"sessionPlacement":   "cookie",
				"sessionIDPlacement": "header",
			},
		}
		liftXhttpSessionIDKeys(stream)
		xhttp := stream["xhttpSettings"].(map[string]any)
		if xhttp["sessionIDPlacement"] != "header" {
			t.Fatalf("explicit new key was overwritten: %v", xhttp["sessionIDPlacement"])
		}
	})

	t.Run("no-op without xhttpSettings or legacy keys", func(t *testing.T) {
		if liftXhttpSessionIDKeys(map[string]any{"wsSettings": map[string]any{}}) {
			t.Fatal("expected no change for non-xhttp stream")
		}
		if liftXhttpSessionIDKeys(map[string]any{"xhttpSettings": map[string]any{"path": "/"}}) {
			t.Fatal("expected no change when no legacy keys present")
		}
	})
}

func TestLiftOutboundsXhttpSessionIDKeys(t *testing.T) {
	raw := json_util.RawMessage(`[{"protocol":"vless","streamSettings":{"network":"xhttp","xhttpSettings":{"sessionKey":"x_session","sessionPlacement":"query"}}}]`)
	out := liftOutboundsXhttpSessionIDKeys(raw)

	var parsed []map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal rewritten outbounds: %v", err)
	}
	xhttp := parsed[0]["streamSettings"].(map[string]any)["xhttpSettings"].(map[string]any)
	if xhttp["sessionIDKey"] != "x_session" || xhttp["sessionIDPlacement"] != "query" {
		t.Fatalf("outbound keys not lifted: %#v", xhttp)
	}
	if _, ok := xhttp["sessionKey"]; ok {
		t.Fatal("legacy sessionKey survived in outbound")
	}

	// Unchanged input must return byte-identical output (no spurious hot-reload).
	clean := json_util.RawMessage(`[{"protocol":"freedom"}]`)
	if got := liftOutboundsXhttpSessionIDKeys(clean); string(got) != string(clean) {
		t.Fatalf("clean outbounds were rewritten: %s", got)
	}
}
