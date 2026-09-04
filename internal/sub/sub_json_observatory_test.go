package sub

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Bad observatory settings (malformed JSON, non-URL destination, bad duration)
// must not leak into the emitted burstObservatory — each falls back to the
// built-in defaults instead of poisoning the client config.
func TestSubJson_ObservatoryConfigInvalidValuesFallBack(t *testing.T) {
	seedSubDB(t)
	inb := seedSubInbound(t, "s1", "lp", 4821, 1, wsTLSStream)
	seedSubBalancer(t, &model.SubBalancer{
		Remark: "pinger", Strategy: "leastPing", InboundIds: []int{inb.Id}, SortOrder: 1, Enabled: true,
	})

	def := defaultSubBalancerObservatoryConfig()
	cases := []struct {
		name string
		cfg  string
	}{
		{"bad json", `{not-json`},
		{"bad destination", `{"destination":"not-a-url"}`},
		{"bad interval", `{"interval":"xyz"}`},
		{"bad timeout", `{"timeout":"5x"}`},
		{"bad connectivity", `{"connectivity":"ftp://bad"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			js := NewSubJsonService("", "", "", "", NewSubService(""))
			js.SetObservatoryConfig(tc.cfg)
			out, _, err := js.GetJson("s1", "req.example.com", true)
			if err != nil {
				t.Fatalf("GetJson: %v", err)
			}
			ping := observatoryPingConfig(t, parseSubJsonDocs(t, out), "pinger")
			if ping["destination"] != def.Destination {
				t.Fatalf("destination = %v, want default %q (cfg=%s)", ping["destination"], def.Destination, tc.cfg)
			}
			if ping["interval"] != def.Interval {
				t.Fatalf("interval = %v, want default %q (cfg=%s)", ping["interval"], def.Interval, tc.cfg)
			}
			if ping["timeout"] != def.Timeout {
				t.Fatalf("timeout = %v, want default %q (cfg=%s)", ping["timeout"], def.Timeout, tc.cfg)
			}
			if ping["connectivity"] != def.Connectivity {
				t.Fatalf("connectivity = %v, want default %q (cfg=%s)", ping["connectivity"], def.Connectivity, tc.cfg)
			}
		})
	}
}
