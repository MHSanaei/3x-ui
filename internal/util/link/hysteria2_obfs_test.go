package link

import (
	"encoding/json"
	"testing"
)

func TestParseHysteria2RestoresSalamanderObfs(t *testing.T) {
	res, err := ParseLink("hysteria2://auth@example.test:4443?sni=example.test&obfs=salamander&obfs-password=secret123#node")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw, err := json.Marshal(res.Outbound["streamSettings"])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var stream struct {
		FinalMask struct {
			UDP []struct {
				Type     string `json:"type"`
				Settings struct {
					Password string `json:"password"`
				} `json:"settings"`
			} `json:"udp"`
		} `json:"finalmask"`
	}
	if err := json.Unmarshal(raw, &stream); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(stream.FinalMask.UDP) != 1 {
		t.Fatalf("finalmask.udp entries = %d, want 1 (obfs was dropped)", len(stream.FinalMask.UDP))
	}
	if got := stream.FinalMask.UDP[0].Type; got != "salamander" {
		t.Fatalf("type = %q, want salamander", got)
	}
	if got := stream.FinalMask.UDP[0].Settings.Password; got != "secret123" {
		t.Fatalf("password = %q, want secret123", got)
	}
}
