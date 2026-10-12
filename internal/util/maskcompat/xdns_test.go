package maskcompat

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/transport/internet/finalmask/xdns"
)

// coreXdnsView runs an xdns mask's settings through conf.XDNS, the loader the core
// calls at startup, and returns the domain names and resolvers the core would use.
func coreXdnsView(t *testing.T, settings any) (domains, resolvers []string, err error) {
	t.Helper()
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal xdns settings: %v", err)
	}
	var mask conf.XDNS
	if err := json.Unmarshal(raw, &mask); err != nil {
		return nil, nil, err
	}
	built, err := mask.Build()
	if err != nil {
		return nil, nil, err
	}
	config := built.(*xdns.Config)
	for _, domain := range config.Domains {
		domains = append(domains, domain.Name)
	}
	for _, resolver := range config.Resolvers {
		resolvers = append(resolvers, resolver.Type+"://"+resolver.Addr)
	}
	return domains, resolvers, nil
}

func TestUpgradeLegacyXdns(t *testing.T) {
	tests := []struct {
		name          string
		mask          string
		want          string
		wantDomains   []string
		wantResolvers []string
	}{
		{
			name:        "bare server domain becomes TXT with the legacy EDNS0 size",
			mask:        `{"type":"xdns","settings":{"domains":["t.example.com"]}}`,
			want:        `{"domains":[{"edns0":1232,"names":["t.example.com"],"types":[16]}]}`,
			wantDomains: []string{"t.example.com"},
		},
		{
			name:        "method suffixes map to their record types",
			mask:        `{"type":"xdns","settings":{"domains":["a.example.com:a","q.example.com:AAAA","t.example.com:txt"]}}`,
			want:        `{"domains":[{"edns0":1232,"names":["a.example.com"],"types":[1]},{"edns0":1232,"names":["q.example.com"],"types":[28]},{"edns0":1232,"names":["t.example.com"],"types":[16]}]}`,
			wantDomains: []string{"a.example.com", "q.example.com", "t.example.com"},
		},
		{
			name:          "client resolver splits into its domain and a udp resolver",
			mask:          `{"type":"XDNS","settings":{"resolvers":["t.example.com:a+udp://8.8.8.8:53"]}}`,
			want:          `{"domains":[{"edns0":1232,"names":["t.example.com"],"types":[1]}],"resolvers":[{"addrs":["udp://8.8.8.8:53"]}]}`,
			wantDomains:   []string{"t.example.com"},
			wantResolvers: []string{"udp://8.8.8.8:53"},
		},
		{
			name:          "a resolver for an already listed domain adds no duplicate",
			mask:          `{"type":"xdns","settings":{"domains":["t.example.com"],"resolvers":["T.example.com+udp://1.1.1.1:53"]}}`,
			want:          `{"domains":[{"edns0":1232,"names":["t.example.com"],"types":[16]}],"resolvers":[{"addrs":["udp://1.1.1.1:53"]}]}`,
			wantDomains:   []string{"t.example.com"},
			wantResolvers: []string{"udp://1.1.1.1:53"},
		},
		{
			name:        "string entries the old core refused are dropped",
			mask:        `{"type":"xdns","settings":{"domains":["t.example.com","m.example.com:mx"],"resolvers":["1.1.1.1:53"]}}`,
			want:        `{"domains":[{"edns0":1232,"names":["t.example.com"],"types":[16]}]}`,
			wantDomains: []string{"t.example.com"},
		},
		{
			name:          "26.9.30 domain and resolver objects move to names and addrs",
			mask:          `{"type":"xdns","settings":{"domains":[{"name":"t.example.com","types":[16,28],"edns0":1232,"lenLimit":200}],"resolvers":[{"type":"tcp","settings":{"addr":"8.8.8.8:53"}},{"type":"udp","settings":{"addr":"1.1.1.1:5353"}}],"extraPoll":1}}`,
			want:          `{"domains":[{"edns0":1232,"lenLimit":200,"names":["t.example.com"],"types":[16,28]}],"extraPoll":1,"resolvers":[{"addrs":["tcp://8.8.8.8:53"]},{"addrs":["udp://1.1.1.1:5353"]}]}`,
			wantDomains:   []string{"t.example.com"},
			wantResolvers: []string{"tcp://8.8.8.8:53", "udp://1.1.1.1:5353"},
		},
		{
			name:          "26.9.30 resolvers the old loader refused are dropped",
			mask:          `{"type":"xdns","settings":{"domains":[{"name":"t.example.com"}],"resolvers":[{"type":"doh","settings":{"addr":"dns.example.com"}},{"type":"udp","settings":{}},{"type":"udp","settings":{"addr":"9.9.9.9:53"}}]}}`,
			want:          `{"domains":[{"names":["t.example.com"]}],"resolvers":[{"addrs":["udp://9.9.9.9:53"]}]}`,
			wantDomains:   []string{"t.example.com"},
			wantResolvers: []string{"udp://9.9.9.9:53"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var mask map[string]any
			if err := json.Unmarshal([]byte(tc.mask), &mask); err != nil {
				t.Fatalf("unmarshal mask: %v", err)
			}
			domains, resolvers, err := coreXdnsView(t, mask["settings"])
			if err == nil && slices.Equal(domains, tc.wantDomains) && slices.Equal(resolvers, tc.wantResolvers) {
				t.Fatal("the core already serves the legacy shape as intended; the upgrade is no longer needed")
			}
			finalmask := map[string]any{"udp": []any{mask}}
			if !UpgradeLegacyXdns(finalmask) {
				t.Fatal("UpgradeLegacyXdns reported no change for a legacy mask")
			}
			got, err := json.Marshal(mask["settings"])
			if err != nil {
				t.Fatalf("marshal upgraded settings: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("upgraded settings\n got: %s\nwant: %s", got, tc.want)
			}
			domains, resolvers, err = coreXdnsView(t, mask["settings"])
			if err != nil {
				t.Fatalf("the core refuses the upgraded settings: %v", err)
			}
			if !slices.Equal(domains, tc.wantDomains) || !slices.Equal(resolvers, tc.wantResolvers) {
				t.Fatalf("the core serves domains %v resolvers %v, want %v %v", domains, resolvers, tc.wantDomains, tc.wantResolvers)
			}
		})
	}
}

func TestUpgradeLegacyXdnsLeavesCurrentShapeAlone(t *testing.T) {
	const current = `{"udp":[{"type":"xdns","settings":{"domains":[{"names":["t.example.com","u.example.com"],"types":[16,28],"edns0":1232}],"resolvers":[{"addrs":["tcp://8.8.8.8:53","1.1.1.1"]}],"extraPoll":2}},{"type":"salamander","settings":{"password":"x"}}]}`
	var finalmask map[string]any
	if err := json.Unmarshal([]byte(current), &finalmask); err != nil {
		t.Fatalf("unmarshal finalmask: %v", err)
	}
	if UpgradeLegacyXdns(finalmask) {
		t.Fatal("UpgradeLegacyXdns rewrote a mask that is already in the names/addrs shape")
	}
	var want map[string]any
	_ = json.Unmarshal([]byte(current), &want)
	got, _ := json.Marshal(finalmask)
	wantJSON, _ := json.Marshal(want)
	if string(got) != string(wantJSON) {
		t.Fatalf("finalmask changed\n got: %s\nwant: %s", got, wantJSON)
	}
}
