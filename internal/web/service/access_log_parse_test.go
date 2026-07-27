package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestParseAccessLogFields(t *testing.T) {
	malformed := []string{
		"",
		"singletoken",
		"2024/01/02",
		"2024/01/02 15:04:05.000000 from",
		"2024/01/02 15:04:05.000000 accepted",
		"2024/01/02 15:04:05.000000 email:",
	}
	for _, line := range malformed {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parseAccessLogFields panicked on %q: %v", line, r)
				}
			}()
			_ = parseAccessLogFields(line)
		}()
	}

	line := "2024/01/02 15:04:05.123456 from 1.2.3.4:555 accepted tcp:example.com:443 [inbound-tag >> outbound-tag] email: alice@example.com"
	entry := parseAccessLogFields(line)
	if entry.FromAddress != "1.2.3.4:555" {
		t.Errorf("FromAddress = %q, want %q", entry.FromAddress, "1.2.3.4:555")
	}
	if entry.ToAddress != "tcp:example.com:443" {
		t.Errorf("ToAddress = %q, want %q", entry.ToAddress, "tcp:example.com:443")
	}
	if entry.Inbound != "inbound-tag" {
		t.Errorf("Inbound = %q, want %q", entry.Inbound, "inbound-tag")
	}
	if entry.Outbound != "outbound-tag" {
		t.Errorf("Outbound = %q, want %q", entry.Outbound, "outbound-tag")
	}
	if entry.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", entry.Email, "alice@example.com")
	}
	if entry.DateTime.IsZero() {
		t.Error("DateTime was not parsed from a well-formed line")
	}
}

func TestAmneziawgEmailIndex(t *testing.T) {
	settings, err := json.Marshal(amneziawg.InboundSettings{
		Server: &amneziawg.ServerSettings{
			PrivateKey: "priv",
			PublicKey:  "pub",
			SubnetIP:   "10.8.1.0",
			SubnetCIDR: 24,
		},
		Clients: []model.Client{
			{Email: "router@x", Enable: true, PublicKey: "pubA", AllowedIPs: []string{"10.8.1.3/32"}},
			{Email: "disabled@x", Enable: false, PublicKey: "pubB", AllowedIPs: []string{"10.8.1.4/32"}},
		},
	})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}

	inbounds := []*model.Inbound{
		{Id: 10, Tag: "in-443-udp", Protocol: model.AmneziaWG, Port: 443, Settings: string(settings)},
		{Id: 11, Tag: "in-443-tcp", Protocol: model.VLESS, Port: 443, Settings: "{}"},
	}

	index := amneziawgEmailIndex(inbounds)

	if got := index["in-443-udp|10.8.1.3"]; got != "router@x" {
		t.Errorf(`index["in-443-udp|10.8.1.3"] = %q, want "router@x"`, got)
	}
	if _, ok := index["in-443-udp|10.8.1.4"]; ok {
		t.Error("a disabled peer must not appear in the index")
	}
	if len(index) != 1 {
		t.Errorf("expected exactly 1 entry (non-amneziawg inbound and disabled peer excluded), got %d: %+v", len(index), index)
	}
}
