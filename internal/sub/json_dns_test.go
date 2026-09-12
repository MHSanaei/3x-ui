package sub

import (
	"testing"
)

func TestSubJsonService_SetDNSConfigDefaults(t *testing.T) {
	svc := NewSubJsonService("", "", "", "", nil)
	svc.SetDNSConfig("", "")

	dns, _ := svc.configJson["dns"].(map[string]any)
	if dns["queryStrategy"] != "UseIP" {
		t.Fatalf("queryStrategy = %v, want UseIP", dns["queryStrategy"])
	}
	servers, _ := dns["servers"].([]any)
	if len(servers) != 1 {
		t.Fatalf("servers len = %d, want 1", len(servers))
	}
	first, _ := servers[0].(map[string]any)
	if first["address"] != "8.8.8.8" || first["skipFallback"] != false {
		t.Fatalf("default server = %#v, want 8.8.8.8 skipFallback=false", first)
	}
}

func TestSubJsonService_SetDNSConfigCustom(t *testing.T) {
	svc := NewSubJsonService("", "", "", "", nil)
	svc.SetDNSConfig("1.1.1.1, 1.0.0.1", "UseIPv4")

	dns, _ := svc.configJson["dns"].(map[string]any)
	if dns["queryStrategy"] != "UseIPv4" {
		t.Fatalf("queryStrategy = %v, want UseIPv4", dns["queryStrategy"])
	}
	servers, _ := dns["servers"].([]any)
	if len(servers) != 2 {
		t.Fatalf("servers len = %d, want 2", len(servers))
	}
	a0, _ := servers[0].(map[string]any)
	a1, _ := servers[1].(map[string]any)
	if a0["address"] != "1.1.1.1" || a1["address"] != "1.0.0.1" {
		t.Fatalf("servers = %#v", servers)
	}
}

func TestSubJsonService_SetDNSConfigInvalidStrategyFallsBack(t *testing.T) {
	svc := NewSubJsonService("", "", "", "", nil)
	svc.SetDNSConfig("9.9.9.9", "not-a-strategy")

	dns, _ := svc.configJson["dns"].(map[string]any)
	if dns["queryStrategy"] != "UseIP" {
		t.Fatalf("queryStrategy = %v, want UseIP fallback", dns["queryStrategy"])
	}
	servers, _ := dns["servers"].([]any)
	first, _ := servers[0].(map[string]any)
	if first["address"] != "9.9.9.9" {
		t.Fatalf("address = %v, want 9.9.9.9", first["address"])
	}
}

func TestParseSubJsonDNSServers(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{"8.8.8.8"}},
		{"  ", []string{"8.8.8.8"}},
		{"1.1.1.1", []string{"1.1.1.1"}},
		{"1.1.1.1,1.0.0.1", []string{"1.1.1.1", "1.0.0.1"}},
		{"1.1.1.1 8.8.8.8", []string{"1.1.1.1", "8.8.8.8"}},
		{`["1.1.1.1","8.8.8.8"]`, []string{"1.1.1.1", "8.8.8.8"}},
		{"https://1.1.1.1/dns-query", []string{"https://1.1.1.1/dns-query"}},
		{"1.1.1.1,1.1.1.1", []string{"1.1.1.1"}},
	}
	for _, tc := range cases {
		got := parseSubJsonDNSServers(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("%q => %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%q => %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}
