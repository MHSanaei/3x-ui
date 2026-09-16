package outbound

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A direct, DNS, loopback or blackhole outbound is not a tunnel, so the probe
// rejects it whatever the spelling: the core resolves "Freedom" to freedom.
func TestTestOutboundsRejectsCaseVariantUntestableIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
		return &stubProcess{cfg: cfg, serveSocks: true}
	})
	withEgressTraceProbe(t, func(*url.URL) *TestEgressResult {
		return &TestEgressResult{IPv4: "198.51.100.2", Country: "ZZ", Warp: "off"}
	})

	tests := []struct {
		name     string
		protocol string
		wantErr  string
	}{
		{"freedom", "freedom", "Direct/DNS outbound cannot be tested"},
		{"Freedom", "Freedom", "Direct/DNS outbound cannot be tested"},
		{"FREEDOM", "FREEDOM", "Direct/DNS outbound cannot be tested"},
		{"DNS", "DNS", "Direct/DNS outbound cannot be tested"},
		{"Blackhole", "Blackhole", "Blocked/blackhole outbound cannot be tested"},
		{"Loopback", "Loopback", "Loopback outbound cannot be tested"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batch := mustJSON(t, []any{map[string]any{"tag": "probe", "protocol": tt.protocol}})
			results, err := (&OutboundService{}).TestOutbounds(batch, srv.URL, "", "http")
			if err != nil {
				t.Fatalf("TestOutbounds: %v", err)
			}
			r := results[0]
			if r.Success || r.Error != tt.wantErr {
				t.Errorf("%q = success=%v err=%q egress=%+v, want the rejection %q",
					tt.protocol, r.Success, r.Error, r.Egress, tt.wantErr)
			}
		})
	}
}
