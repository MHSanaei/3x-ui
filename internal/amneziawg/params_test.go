package amneziawg

import (
	"fmt"
	"strings"
	"testing"
)

func TestGenerateAWGParamsJcRange(t *testing.T) {
	for i := 0; i < 20; i++ {
		p := GenerateAWGParams()
		if p.Jc < 4 || p.Jc > 6 {
			t.Fatalf("Jc = %d, want in [4,6]", p.Jc)
		}
	}
}

func TestGenerateAWGParamsDefaults(t *testing.T) {
	p := GenerateAWGParams()
	if p.Jmin != 10 {
		t.Fatalf("Jmin = %d, want 10", p.Jmin)
	}
	if p.Jmax != 50 {
		t.Fatalf("Jmax = %d, want 50", p.Jmax)
	}
	if p.SubnetIP != DefaultSubnetIP {
		t.Fatalf("SubnetIP = %q, want %q", p.SubnetIP, DefaultSubnetIP)
	}
	if p.SubnetCIDR != DefaultSubnetCIDR {
		t.Fatalf("SubnetCIDR = %d, want %d", p.SubnetCIDR, DefaultSubnetCIDR)
	}
	if p.ServerPort != DefaultServerPort {
		t.Fatalf("ServerPort = %d, want %d", p.ServerPort, DefaultServerPort)
	}
	if p.PrimaryDNS != DefaultPrimaryDNS {
		t.Fatalf("PrimaryDNS = %q, want %q", p.PrimaryDNS, DefaultPrimaryDNS)
	}
	if p.SecondaryDNS != DefaultSecondaryDNS {
		t.Fatalf("SecondaryDNS = %q, want %q", p.SecondaryDNS, DefaultSecondaryDNS)
	}
}

func TestGenerateAWGParamsSValuesDistinct(t *testing.T) {
	for i := 0; i < 20; i++ {
		p := GenerateAWGParams()
		seen := map[int]bool{p.S1: true, p.S2: true, p.S3: true, p.S4: true}
		if len(seen) != 4 {
			t.Fatalf("S1-S4 must be distinct: got %d unique, want 4", len(seen))
		}
	}
}

func TestGenerateAWGParamsSValuesRanges(t *testing.T) {
	p := GenerateAWGParams()
	if p.S1 < 15 || p.S1 > 1500 {
		t.Fatalf("S1 = %d, want in [15,1500]", p.S1)
	}
	if p.S2 < 15 || p.S2 > 1500 {
		t.Fatalf("S2 = %d, want in [15,1500]", p.S2)
	}
	if p.S3 < 10 || p.S3 > 500 {
		t.Fatalf("S3 = %d, want in [10,500]", p.S3)
	}
	if p.S4 < 5 || p.S4 > 200 {
		t.Fatalf("S4 = %d, want in [5,200]", p.S4)
	}
}

func TestGenerateAWGParamsHeadersRangeFormat(t *testing.T) {
	for i := 0; i < 20; i++ {
		p := GenerateAWGParams()
		for _, h := range []string{p.H1, p.H2, p.H3, p.H4} {
			parts := strings.Split(h, "-")
			if len(parts) != 2 {
				t.Fatalf("header %q does not match range format", h)
			}
			if parts[0] == "" || parts[1] == "" {
				t.Fatalf("header %q has empty part", h)
			}
		}
	}
}

func TestGenerateAWGParamsHeadersNonOverlapping(t *testing.T) {
	for i := 0; i < 20; i++ {
		p := GenerateAWGParams()
		headers := []struct {
			name string
			val  string
		}{
			{"H1", p.H1}, {"H2", p.H2}, {"H3", p.H3}, {"H4", p.H4},
		}
		for j, h := range headers {
			parts := strings.Split(h.val, "-")
			start := parseHeadInt(parts[0])
			end := parseHeadInt(parts[1])
			if start > end {
				t.Fatalf("%s: start %d > end %d", h.name, start, end)
			}
			if j == 0 {
				continue
			}
			prevParts := strings.Split(headers[j-1].val, "-")
			prevEnd := parseHeadInt(prevParts[1])
			if start <= prevEnd {
				t.Fatalf("%s start %d must be > %s end %d", h.name, start, headers[j-1].name, prevEnd)
			}
		}
	}
}

func parseHeadInt(s string) int64 {
	var v int64
	for _, c := range s {
		v = v*10 + int64(c-'0')
	}
	return v
}

func TestNextClientIP(t *testing.T) {
	tests := []struct {
		name     string
		subnetIP string
		assigned []string
		want     string
		wantErr  bool
	}{
		{name: "empty returns .2", subnetIP: "10.8.1.0", assigned: nil, want: "10.8.1.2"},
		{name: "skips .1 when base ends in .0", subnetIP: "10.8.1.0", assigned: []string{"10.8.1.2"}, want: "10.8.1.3"},
		{name: "non .0 base starts at .1", subnetIP: "10.8.1.1", assigned: nil, want: "10.8.1.2"},
		{name: "appends after highest", subnetIP: "10.8.1.0", assigned: []string{"10.8.1.3", "10.8.1.4"}, want: "10.8.1.5"},
		{name: "handles offset", subnetIP: "192.168.1.0", assigned: nil, want: "192.168.1.2"},
		{name: "exhausted at 254", subnetIP: "10.0.0.0", assigned: ipRange("10.0.0.", 2, 254), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NextClientIP(tt.subnetIP, tt.assigned)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func ipRange(prefix string, start, end int) []string {
	var r []string
	for i := start; i <= end; i++ {
		r = append(r, fmt.Sprintf("%s%d", prefix, i))
	}
	return r
}

func TestNextClientIPInvalidSubnet(t *testing.T) {
	_, err := NextClientIP("not-an-ip", nil)
	if err == nil {
		t.Fatal("expected error for invalid subnet IP")
	}
}

func TestNextClientIPIgnoresBadAssigned(t *testing.T) {
	got, err := NextClientIP("10.8.1.0", []string{"not-an-ip"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "10.8.1.2" {
		t.Fatalf("got %q, want %q", got, "10.8.1.2")
	}
}

func TestNextClientIPSingleClient(t *testing.T) {
	got, err := NextClientIP("10.8.1.0", []string{"10.8.1.2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "10.8.1.3" {
		t.Fatalf("got %q, want %q", got, "10.8.1.3")
	}
}

func TestGenerateAWGParamsDeterministicVariation(t *testing.T) {
	seenJc := make(map[int]bool)
	for i := 0; i < 10; i++ {
		p := GenerateAWGParams()
		seenJc[p.Jc] = true
	}
	if len(seenJc) < 1 {
		t.Fatal("params appear broken")
	}
}