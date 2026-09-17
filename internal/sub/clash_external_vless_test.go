package sub

import (
	"testing"
)

// #6572: a merged external VLESS node carrying VLESS encryption must keep
// the field, like the standalone inbound path (buildProxy) already does.
func TestClashProxyFromExternalVlessKeepsEncryption(t *testing.T) {
	link := "vless://00000000-0000-0000-0000-000000000001@example.com:443?type=tcp&security=reality&pbk=PBK&fp=chrome&sni=example.org&sid=00&flow=xtls-rprx-vision&encryption=mlkem768x25519plus.native.0rtt#ext"
	svc := NewSubClashService(false, "", NewSubService(""))
	proxy := svc.clashProxyFromExternal(link, "ext")
	if proxy == nil {
		t.Fatal("expected a clash proxy, got nil")
	}
	if proxy["type"] != "vless" {
		t.Fatalf("type = %v, want vless", proxy["type"])
	}
	if proxy["uuid"] != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("uuid = %v", proxy["uuid"])
	}
	if proxy["encryption"] != "mlkem768x25519plus.native.0rtt" {
		t.Fatalf("encryption = %v, want mlkem768x25519plus.native.0rtt", proxy["encryption"])
	}
}

func TestClashProxyFromExternalVlessOmitsEmptyEncryption(t *testing.T) {
	link := "vless://00000000-0000-0000-0000-000000000002@example.com:443?type=tcp&security=reality&pbk=PBK&fp=chrome&sni=example.org&sid=00#ext"
	svc := NewSubClashService(false, "", NewSubService(""))
	proxy := svc.clashProxyFromExternal(link, "ext")
	if proxy == nil {
		t.Fatal("expected a clash proxy, got nil")
	}
	if _, ok := proxy["encryption"]; ok {
		t.Fatalf("encryption should be omitted, got %v", proxy["encryption"])
	}
}
