package conf_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	. "github.com/xtls/xray-core/infra/conf"
)

func TestStringListUnmarshalError(t *testing.T) {
	rawJSON := `1234`
	list := new(StringList)
	err := json.Unmarshal([]byte(rawJSON), list)
	if err == nil {
		t.Error("expected error, but got nil")
	}
}

func TestStringListLen(t *testing.T) {
	rawJSON := `"a, b, c, d"`
	var list StringList
	err := json.Unmarshal([]byte(rawJSON), &list)
	common.Must(err)
	if r := cmp.Diff([]string(list), []string{"a", " b", " c", " d"}); r != "" {
		t.Error(r)
	}
}

func TestIPParsing(t *testing.T) {
	rawJSON := "\"8.8.8.8\""
	var address Address
	err := json.Unmarshal([]byte(rawJSON), &address)
	common.Must(err)
	if r := cmp.Diff(address.IP(), net.IP{8, 8, 8, 8}); r != "" {
		t.Error(r)
	}
}

func TestDomainParsing(t *testing.T) {
	rawJSON := "\"example.com\""
	var address Address
	common.Must(json.Unmarshal([]byte(rawJSON), &address))
	if address.Domain() != "example.com" {
		t.Error("domain: ", address.Domain())
	}
}

func TestURLParsing(t *testing.T) {
	{
		rawJSON := "\"https://dns.google/dns-query\""
		var address Address
		common.Must(json.Unmarshal([]byte(rawJSON), &address))
		if address.Domain() != "https://dns.google/dns-query" {
			t.Error("URL: ", address.Domain())
		}
	}
	{
		rawJSON := "\"https+local://dns.google/dns-query\""
		var address Address
		common.Must(json.Unmarshal([]byte(rawJSON), &address))
		if address.Domain() != "https+local://dns.google/dns-query" {
			t.Error("URL: ", address.Domain())
		}
	}
}

func TestInvalidAddressJson(t *testing.T) {
	rawJSON := "1234"
	var address Address
	err := json.Unmarshal([]byte(rawJSON), &address)
	if err == nil {
		t.Error("nil error")
	}
}

func TestStringNetwork(t *testing.T) {
	var network Network
	common.Must(json.Unmarshal([]byte(`"tcp"`), &network))
	if v := network.Build(); v != net.Network_TCP {
		t.Error("network: ", v)
	}
}

func TestArrayNetworkList(t *testing.T) {
	var list NetworkList
	common.Must(json.Unmarshal([]byte("[\"Tcp\"]"), &list))

	nlist := list.Build()
	if !net.HasNetwork(nlist, net.Network_TCP) {
		t.Error("no tcp network")
	}
	if net.HasNetwork(nlist, net.Network_UDP) {
		t.Error("has udp network")
	}
}

func TestStringNetworkList(t *testing.T) {
	var list NetworkList
	common.Must(json.Unmarshal([]byte("\"TCP, ip\""), &list))

	nlist := list.Build()
	if !net.HasNetwork(nlist, net.Network_TCP) {
		t.Error("no tcp network")
	}
	if net.HasNetwork(nlist, net.Network_UDP) {
		t.Error("has udp network")
	}
}

func TestInvalidNetworkJson(t *testing.T) {
	var list NetworkList
	err := json.Unmarshal([]byte("0"), &list)
	if err == nil {
		t.Error("nil error")
	}
}

func TestIntPort(t *testing.T) {
	var portRange PortRange
	common.Must(json.Unmarshal([]byte("1234"), &portRange))

	if r := cmp.Diff(portRange, PortRange{
		From: 1234, To: 1234,
	}); r != "" {
		t.Error(r)
	}
}

func TestOverRangeIntPort(t *testing.T) {
	var portRange PortRange
	err := json.Unmarshal([]byte("70000"), &portRange)
	if err == nil {
		t.Error("nil error")
	}

	err = json.Unmarshal([]byte("-1"), &portRange)
	if err == nil {
		t.Error("nil error")
	}
}

func TestEnvPort(t *testing.T) {
	common.Must(os.Setenv("PORT", "1234"))

	var portRange PortRange
	common.Must(json.Unmarshal([]byte("\"env:PORT\""), &portRange))

	if r := cmp.Diff(portRange, PortRange{
		From: 1234, To: 1234,
	}); r != "" {
		t.Error(r)
	}
}

func TestSingleStringPort(t *testing.T) {
	var portRange PortRange
	common.Must(json.Unmarshal([]byte("\"1234\""), &portRange))

	if r := cmp.Diff(portRange, PortRange{
		From: 1234, To: 1234,
	}); r != "" {
		t.Error(r)
	}
}

func TestStringPairPort(t *testing.T) {
	var portRange PortRange
	common.Must(json.Unmarshal([]byte("\"1234-5678\""), &portRange))

	if r := cmp.Diff(portRange, PortRange{
		From: 1234, To: 5678,
	}); r != "" {
		t.Error(r)
	}
}

func TestOverRangeStringPort(t *testing.T) {
	var portRange PortRange
	err := json.Unmarshal([]byte("\"65536\""), &portRange)
	if err == nil {
		t.Error("nil error")
	}

	err = json.Unmarshal([]byte("\"70000-80000\""), &portRange)
	if err == nil {
		t.Error("nil error")
	}

	err = json.Unmarshal([]byte("\"1-90000\""), &portRange)
	if err == nil {
		t.Error("nil error")
	}

	err = json.Unmarshal([]byte("\"700-600\""), &portRange)
	if err == nil {
		t.Error("nil error")
	}
}

func TestUserParsing(t *testing.T) {
	user := new(User)
	common.Must(json.Unmarshal([]byte(`{
    "id": "96edb838-6d68-42ef-a933-25f7ac3a9d09",
    "email": "love@example.com",
    "level": 1
  }`), user))

	nUser := user.Build()
	if r := cmp.Diff(nUser, &protocol.User{
		Level: 1,
		Email: "love@example.com",
	}, cmpopts.IgnoreUnexported(protocol.User{})); r != "" {
		t.Error(r)
	}
}

func TestInvalidUserJson(t *testing.T) {
	user := new(User)
	err := json.Unmarshal([]byte(`{"email": 1234}`), user)
	if err == nil {
		t.Error("nil error")
	}
}
