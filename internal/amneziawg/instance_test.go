package amneziawg

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func mkInboundSettings(t *testing.T, server *ServerSettings, clients []model.Client) string {
	t.Helper()
	bs, err := json.Marshal(InboundSettings{Server: server, Clients: clients})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	return string(bs)
}

func validServer() *ServerSettings {
	return &ServerSettings{
		PrivateKey: "serverPriv",
		PublicKey:  "serverPub",
		SubnetIP:   "10.8.1.0",
		SubnetCIDR: 24,
	}
}

func TestInstanceFromInboundParsesEnabledPeers(t *testing.T) {
	settings := mkInboundSettings(t, validServer(), []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pubA", PreSharedKey: "pskA", AllowedIPs: []string{"10.8.1.2/32"}},
		{Email: "b@x", Enable: false, PublicKey: "pubB", AllowedIPs: []string{"10.8.1.3/32"}},
		{Email: "c@x", Enable: true, PublicKey: "", AllowedIPs: []string{"10.8.1.4/32"}}, // no key: skipped
		{Email: "d@x", Enable: true, PublicKey: "pubD", AllowedIPs: nil},                 // no address: skipped
	})
	ib := &model.Inbound{Id: 7, Tag: "awg-tag", Protocol: model.AmneziaWG, Port: 51820, Settings: settings}

	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.Id != 7 || inst.Tag != "awg-tag" || inst.ListenPort != 51820 {
		t.Fatalf("instance identity not carried over: %+v", inst)
	}
	if inst.InterfaceName != "awg7" {
		t.Fatalf("InterfaceName = %q, want awg7", inst.InterfaceName)
	}
	if len(inst.Address) != 1 || inst.Address[0] != "10.8.1.1/24" {
		t.Fatalf("Address = %v, want [10.8.1.1/24]", inst.Address)
	}
	if len(inst.Peers) != 1 {
		t.Fatalf("Peers = %+v, want exactly 1 (only a@x qualifies)", inst.Peers)
	}
	p := inst.Peers[0]
	if p.Email != "a@x" || p.PublicKey != "pubA" || p.PresharedKey != "pskA" || len(p.AllowedIPs) != 1 || p.AllowedIPs[0] != "10.8.1.2/32" {
		t.Fatalf("peer mismatch: %+v", p)
	}
}

func TestInstanceFromInboundRejectsWrongProtocol(t *testing.T) {
	settings := mkInboundSettings(t, validServer(), []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pubA", AllowedIPs: []string{"10.8.1.2/32"}},
	})
	ib := &model.Inbound{Id: 1, Protocol: model.VLESS, Settings: settings}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("non-AmneziaWG inbound must be rejected")
	}
}

func TestInstanceFromInboundRejectsNil(t *testing.T) {
	if _, ok := InstanceFromInbound(nil); ok {
		t.Fatal("nil inbound must be rejected")
	}
}

func TestInstanceFromInboundRejectsMissingServer(t *testing.T) {
	ib := &model.Inbound{Id: 1, Protocol: model.AmneziaWG, Settings: `{"clients":[]}`}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("settings with no server block must be rejected")
	}
}

func TestInstanceFromInboundRejectsUnparseableSettings(t *testing.T) {
	ib := &model.Inbound{Id: 1, Protocol: model.AmneziaWG, Settings: `not json`}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("unparseable settings must be rejected")
	}
}

func TestInstanceFromInboundEmptyWhenNoEnabledPeers(t *testing.T) {
	settings := mkInboundSettings(t, validServer(), []model.Client{
		{Email: "a@x", Enable: false, PublicKey: "pubA", AllowedIPs: []string{"10.8.1.2/32"}},
	})
	ib := &model.Inbound{Id: 1, Protocol: model.AmneziaWG, Settings: settings}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("an inbound with zero enabled peers must be skipped, like mtproto.InstanceFromInbound")
	}
}

func TestServerAddress(t *testing.T) {
	cases := []struct {
		subnet string
		cidr   int
		want   string
	}{
		{"10.8.1.0", 24, "10.8.1.1/24"},
		{"10.8.1.0", 0, "10.8.1.1/24"},  // cidr <= 0 defaults to /24
		{"10.8.1.5", 24, "10.8.1.1/24"}, // non-network base: must not collide with peer allocation starting at .2
		{"10.8.1.254", 24, "10.8.1.1/24"},
		{"192.168.5.10", 32, "192.168.5.10/32"}, // /32 has no host bits: used as-is
	}
	for _, c := range cases {
		if got := serverAddress(c.subnet, c.cidr); got != c.want {
			t.Errorf("serverAddress(%q, %d) = %q, want %q", c.subnet, c.cidr, got, c.want)
		}
	}
}

func TestInterfaceNameForID(t *testing.T) {
	if got := interfaceNameForID(42); got != "awg42" {
		t.Errorf("interfaceNameForID(42) = %q, want awg42", got)
	}
}
