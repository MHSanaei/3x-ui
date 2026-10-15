package psiphon

import (
	"context"
	"testing"
)

func TestParseTrace(t *testing.T) {
	raw := `fl=996f56
h=www.cloudflare.com
ip=198.51.100.23
ts=1792019615.000
visit_scheme=https
loc=DE
warp=off
`
	ip, loc := parseTrace(raw)
	if ip != "198.51.100.23" {
		t.Errorf("parseTrace ip = %q, want 198.51.100.23", ip)
	}
	if loc != "DE" {
		t.Errorf("parseTrace loc = %q, want DE", loc)
	}
}

func TestProbeInvalidPort(t *testing.T) {
	ctx := context.Background()
	res := Probe(ctx, -1)
	if res.OK {
		t.Errorf("expected failure for invalid port, got OK")
	}
	if res.Error == "" {
		t.Errorf("expected error message for invalid port")
	}
}
