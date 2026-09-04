package amneziawgnet

import (
	"encoding/json"
	"testing"
)

func TestBuildSocksBridge_BridgesAndPreservesSiblings(t *testing.T) {
	raw := []byte(`{
		"protocol": "amneziawg",
		"tag": "awg-hop",
		"sendThrough": "0.0.0.0",
		"targetStrategy": {"strategy": "UseIPv4"},
		"mux": {"enabled": false},
		"streamSettings": {"sockopt": {"tcpFastOpen": true}},
		"settings": {"secretKey": "x"}
	}`)
	out, ok := BuildSocksBridge(raw)
	if !ok {
		t.Fatal("valid entry rejected")
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got["protocol"] != "socks" {
		t.Fatalf("protocol = %v", got["protocol"])
	}
	if got["tag"] != "awg-hop" || got["sendThrough"] != "0.0.0.0" {
		t.Fatalf("siblings dropped: %v", got)
	}
	if _, ok := got["targetStrategy"].(map[string]any); !ok {
		t.Fatalf("targetStrategy dropped: %v", got["targetStrategy"])
	}
	settings, _ := got["settings"].(map[string]any)
	if settings == nil || settings["user"] != "awg-hop" || settings["address"] != "127.0.0.1" {
		t.Fatalf("bridge settings wrong: %v", settings)
	}
}

func TestBuildSocksBridge_RejectsUnusableTags(t *testing.T) {
	for name, raw := range map[string][]byte{
		"missing tag":    []byte(`{"protocol":"amneziawg","settings":{}}`),
		"empty tag":      []byte(`{"protocol":"amneziawg","tag":"","settings":{}}`),
		"non-string tag": []byte(`{"protocol":"amneziawg","tag":123,"settings":{}}`),
		"not an object":  []byte(`[1,2,3]`),
	} {
		if _, ok := BuildSocksBridge(raw); ok {
			t.Fatalf("%s: expected rejection", name)
		}
	}
}
