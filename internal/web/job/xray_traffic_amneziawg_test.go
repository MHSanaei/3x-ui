package job

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// An AmneziaWG bridge reuses its inbound's own tag, so Xray reports the same
// bytes AmneziaWGJob already reported from `awg show dump`; both call
// AddTraffic, which accumulates, so the inbound total came out roughly double.
func TestDropAmneziawgBridgeTraffic(t *testing.T) {
	rows := []*xray.Traffic{
		{IsInbound: true, Tag: "awg-1", Up: 100, Down: 200},
		{IsInbound: true, Tag: "vless-in", Up: 1, Down: 2},
		{IsOutbound: true, Tag: "awg-1", Up: 7, Down: 8},
		nil,
		{IsInbound: true, Tag: "awg-2", Up: 5, Down: 6},
	}
	bridges := map[string]struct{}{"awg-1": {}}

	kept := dropAmneziawgBridgeTraffic(rows, bridges)

	var sawBridgeInbound, sawBridgeOutbound, sawOther, sawUnrouted bool
	for _, tr := range kept {
		if tr == nil {
			continue
		}
		switch {
		case tr.IsInbound && tr.Tag == "awg-1":
			sawBridgeInbound = true
		case tr.IsOutbound && tr.Tag == "awg-1":
			sawBridgeOutbound = true
		case tr.Tag == "vless-in":
			sawOther = true
		case tr.Tag == "awg-2":
			sawUnrouted = true
		}
	}
	if sawBridgeInbound {
		t.Error("the bridge's inbound row must be dropped; it duplicates the awg dump counters")
	}
	if !sawBridgeOutbound {
		t.Error("only inbound stats collide: an outbound row sharing the tag must survive")
	}
	if !sawOther {
		t.Error("an unrelated inbound must be untouched")
	}
	if !sawUnrouted {
		t.Error("an AmneziaWG inbound with no bridge has no Xray row to duplicate, so nothing to drop")
	}
	if len(kept) != len(rows)-1 {
		t.Fatalf("kept %d rows, want %d (exactly one dropped)", len(kept), len(rows)-1)
	}
}

func TestDropAmneziawgBridgeTrafficNoBridgesIsIdentity(t *testing.T) {
	rows := []*xray.Traffic{{IsInbound: true, Tag: "awg-1", Up: 1}}
	if got := dropAmneziawgBridgeTraffic(rows, nil); len(got) != 1 {
		t.Fatalf("with no bridges the slice must pass through untouched, got %d rows", len(got))
	}
}
