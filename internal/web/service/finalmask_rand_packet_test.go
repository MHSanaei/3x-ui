package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func streamWithNoiseItem(t *testing.T, item map[string]any) map[string]any {
	t.Helper()
	return map[string]any{
		"network": "tcp",
		"finalmask": map[string]any{
			"udp": []any{map[string]any{
				"type": "noise",
				"settings": map[string]any{
					"reset": "60",
					"noise": []any{item},
				},
			}},
		},
	}
}

func noiseItem(t *testing.T, stream map[string]any) map[string]any {
	t.Helper()
	finalmask, _ := stream["finalmask"].(map[string]any)
	udp, _ := finalmask["udp"].([]any)
	mask, _ := udp[0].(map[string]any)
	settings, _ := mask["settings"].(map[string]any)
	noise, _ := settings["noise"].([]any)
	item, _ := noise[0].(map[string]any)
	return item
}

// TestDropEmptyRandPacketsClearsEditorResidue is the regression for the mask
// editor writing packet:[] alongside a rand. xray-core counts the empty array
// as a packet and refuses the config, which keeps every inbound offline.
func TestDropEmptyRandPacketsClearsEditorResidue(t *testing.T) {
	stream := streamWithNoiseItem(t, map[string]any{
		"type":   "array",
		"rand":   "1-8192",
		"packet": []any{},
		"delay":  "5",
	})

	if cleared := dropEmptyRandPackets(stream["finalmask"]); cleared != 1 {
		t.Fatalf("cleared = %d, want 1", cleared)
	}
	item := noiseItem(t, stream)
	if _, present := item["packet"]; present {
		t.Fatalf("packet survived: %#v", item)
	}
	if item["rand"] != "1-8192" || item["delay"] != "5" {
		t.Fatalf("healing changed the mask: %#v", item)
	}
}

func TestDropEmptyRandPacketsLeavesRealPacketsAlone(t *testing.T) {
	tests := []struct {
		name string
		item map[string]any
	}{
		{"packet without a rand", map[string]any{"type": "array", "packet": []any{1.0, 2.0}, "rand": 0.0}},
		{"empty packet without a rand", map[string]any{"type": "array", "packet": []any{}}},
		{"empty packet with a zero rand", map[string]any{"type": "array", "packet": []any{}, "rand": 0.0}},
		{"empty packet with a zero range", map[string]any{"type": "array", "packet": []any{}, "rand": "0-0"}},
		{"string packet", map[string]any{"type": "str", "packet": "ping", "rand": "1-10"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := streamWithNoiseItem(t, tt.item)
			if cleared := dropEmptyRandPackets(stream["finalmask"]); cleared != 0 {
				t.Fatalf("cleared = %d, want the item left alone", cleared)
			}
			if _, present := noiseItem(t, stream)["packet"]; !present {
				t.Fatal("packet was dropped")
			}
		})
	}
}

// TestDropEmptyRandPacketsReachesNestedItems covers header-custom, whose items
// sit two arrays deep and are subject to the same exclusive-kind rule.
func TestDropEmptyRandPacketsReachesNestedItems(t *testing.T) {
	stream := map[string]any{
		"finalmask": map[string]any{
			"tcp": []any{map[string]any{
				"type": "header-custom",
				"settings": map[string]any{
					"clients": []any{[]any{map[string]any{"type": "array", "rand": 64.0, "packet": []any{}}}},
					"servers": []any{[]any{map[string]any{"type": "array", "rand": 32.0, "packet": []any{}}}},
				},
			}},
		},
	}
	if cleared := dropEmptyRandPackets(stream["finalmask"]); cleared != 2 {
		t.Fatalf("cleared = %d, want 2", cleared)
	}
}

func TestDropEmptyRandPacketsIgnoresMissingFinalMask(t *testing.T) {
	stream := map[string]any{"network": "tcp"}
	if cleared := dropEmptyRandPackets(stream["finalmask"]); cleared != 0 {
		t.Fatalf("cleared = %d, want 0", cleared)
	}
}

// TestHealedConfigsBuildInXray closes the loop on both heals: the rows xray
// refuses outright must build once the panel has healed them.
func TestHealedConfigsBuildInXray(t *testing.T) {
	t.Run("hysteria v1 row", func(t *testing.T) {
		in := model.Inbound{
			Protocol:       model.Hysteria,
			Port:           36715,
			Listen:         "127.0.0.1",
			Tag:            "in-hysteria",
			Settings:       `{"version":1,"clients":[{"auth":"tok","email":"a@x"}]}`,
			StreamSettings: `{"network":"hysteria","hysteriaSettings":{"version":1,"udpIdleTimeout":60}}`,
		}

		raw, err := json.Marshal(in.GenXrayInboundConfig())
		if err != nil {
			t.Fatalf("marshal generated inbound: %v", err)
		}
		var healed map[string]any
		if err := json.Unmarshal(raw, &healed); err != nil {
			t.Fatalf("decode generated inbound: %v", err)
		}
		assertXrayAccepts(t, "the healed hysteria inbound", buildGoldenInbound(t, healed))

		var unhealed map[string]any
		if err := json.Unmarshal([]byte(`{
			"tag":"in-hysteria","listen":"127.0.0.1","port":36715,"protocol":"hysteria",
			"settings":`+in.Settings+`,"streamSettings":`+in.StreamSettings+`}`), &unhealed); err != nil {
			t.Fatalf("decode raw inbound: %v", err)
		}
		if err := buildGoldenInbound(t, unhealed); err == nil {
			t.Fatal("the unhealed v1 row is expected to be refused; the heal is what makes it buildable")
		}
	})

	t.Run("noise item with an empty packet", func(t *testing.T) {
		item := map[string]any{"type": "array", "rand": "1-8192", "packet": []any{}, "delay": "5"}
		stream := streamWithNoiseItem(t, item)
		inbound := map[string]any{
			"tag": "in-vless", "listen": "127.0.0.1", "port": 8443, "protocol": "vless",
			"settings":       map[string]any{"clients": []any{}, "decryption": "none"},
			"streamSettings": stream,
		}
		if err := buildGoldenInbound(t, inbound); err == nil {
			t.Fatal("xray-core is expected to refuse a packet and a rand on one item")
		}

		dropEmptyRandPackets(stream["finalmask"])
		inbound["streamSettings"] = stream
		assertXrayAccepts(t, "the healed noise mask", buildGoldenInbound(t, inbound))
	})
}
