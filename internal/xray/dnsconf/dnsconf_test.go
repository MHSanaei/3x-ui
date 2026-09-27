package dnsconf

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantErr   string
		wantNil   bool
		wantCount int
	}{
		{name: "blank", value: "  ", wantNil: true},
		{name: "array of strings", value: `["1.1.1.1", "tls://1.0.0.1"]`, wantCount: 2},
		{name: "array with object entry", value: `[{"address": "1.1.1.1", "domains": ["geosite:youtube"]}]`, wantCount: 1},
		{name: "object with hosts and strategy", value: `{"queryStrategy": "UseIPv4", "hosts": {"example.com": "1.2.3.4"}, "servers": ["https://dns.google/dns-query"]}`, wantCount: 1},
		{name: "hosts value list", value: `{"hosts": {"example.com": ["1.2.3.4", "5.6.7.8"]}, "servers": ["1.1.1.1"]}`, wantCount: 1},
		{name: "comma separated domains", value: `[{"address": "1.1.1.1", "domains": "geosite:youtube,geosite:netflix"}]`, wantCount: 1},
		{name: "client ip set", value: `{"clientIp": "1.2.3.4", "servers": [{"address": "1.1.1.1", "clientIp": "2001:db8::1"}]}`, wantCount: 1},

		{name: "malformed JSON", value: `[`, wantErr: "invalid DNS JSON"},
		{name: "scalar", value: `42`, wantErr: "must be a JSON object or an array of servers"},
		{name: "empty array", value: `[]`, wantErr: `"servers" must list at least one DNS server`},
		{name: "object without servers", value: `{"hosts": {"a": "b"}}`, wantErr: `"servers" must list at least one DNS server`},
		{name: "misspelled servers key", value: `{"server": ["1.1.1.1"]}`, wantErr: `"servers" must list at least one DNS server`},
		{name: "non-string server entry", value: `[53]`, wantErr: "invalid DNS config"},
		{name: "server without address", value: `[{"skipFallback": true}]`, wantErr: `needs a non-empty "address"`},
		{name: "empty server string", value: `[""]`, wantErr: "is empty"},
		{name: "empty server address", value: `[{"address": "  "}]`, wantErr: `needs a non-empty "address"`},
		{name: "server address wrong type", value: `[{"address": 53}]`, wantErr: "invalid DNS config"},
		{name: "hosts wrong type", value: `{"hosts": 5, "servers": ["1.1.1.1"]}`, wantErr: "invalid DNS config"},
		{name: "strategy wrong type", value: `{"queryStrategy": 123, "servers": ["1.1.1.1"]}`, wantErr: "invalid DNS config"},
		{name: "client ip not an address", value: `{"clientIp": "not-an-ip", "servers": ["1.1.1.1"]}`, wantErr: "clientIp must be an IP address"},
		{name: "server client ip not an address", value: `[{"address": "1.1.1.1", "clientIp": "example.com"}]`, wantErr: "clientIp must be an IP address"},
		{name: "server port as string", value: `[{"address": "1.1.1.1", "port": "53"}]`, wantErr: "invalid DNS config"},
		{name: "server domains wrong type", value: `[{"address": "1.1.1.1", "domains": 5}]`, wantErr: "invalid DNS config"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			block, err := Parse(tc.value)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if block != nil {
					t.Fatalf("block = %v, want nil", block)
				}
				return
			}
			if block == nil {
				t.Fatal("block = nil")
			}
			servers, _ := block["servers"].([]any)
			if len(servers) != tc.wantCount {
				t.Fatalf("servers = %v, want %d", servers, tc.wantCount)
			}
		})
	}
}
