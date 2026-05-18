package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestNormalizeBasePath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "/"},
		{"   ", "/"},
		{"/", "/"},
		{"/panel", "/panel/"},
		{"panel", "/panel/"},
		{"panel/", "/panel/"},
		{"/panel/", "/panel/"},
		{"  /panel  ", "/panel/"},
		{"/a/b/c", "/a/b/c/"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := normalizeBasePath(c.in)
			if got != c.want {
				t.Fatalf("normalizeBasePath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNodeMetricKey(t *testing.T) {
	cases := []struct {
		id     int
		metric string
		want   string
	}{
		{1, "cpu", "node:1:cpu"},
		{42, "mem", "node:42:mem"},
		{0, "anything", "node:0:anything"},
	}
	for _, c := range cases {
		got := nodeMetricKey(c.id, c.metric)
		if got != c.want {
			t.Fatalf("nodeMetricKey(%d, %q) = %q, want %q", c.id, c.metric, got, c.want)
		}
	}
}

func TestHeartbeatPatch_ToUI_OnlineCopiesFields(t *testing.T) {
	p := HeartbeatPatch{
		Status:       "ignored-source",
		LatencyMs:    42,
		XrayVersion:  "1.8.4",
		PanelVersion: "3.0.0",
		CpuPct:       12.5,
		MemPct:       33.3,
		UptimeSecs:   12345,
		LastError:    "",
	}
	ui := p.ToUI(true)
	if ui.Status != "online" {
		t.Fatalf("Status = %q, want online", ui.Status)
	}
	if ui.LatencyMs != 42 || ui.XrayVersion != "1.8.4" || ui.PanelVersion != "3.0.0" {
		t.Fatalf("scalar copy mismatch: %+v", ui)
	}
	if ui.CpuPct != 12.5 || ui.MemPct != 33.3 || ui.UptimeSecs != 12345 {
		t.Fatalf("metric copy mismatch: %+v", ui)
	}
	if ui.Error != "" {
		t.Fatalf("Error = %q, want empty", ui.Error)
	}
}

func TestHeartbeatPatch_ToUI_OfflinePreservesError(t *testing.T) {
	p := HeartbeatPatch{LastError: "connection refused"}
	ui := p.ToUI(false)
	if ui.Status != "offline" {
		t.Fatalf("Status = %q, want offline", ui.Status)
	}
	if ui.Error != "connection refused" {
		t.Fatalf("Error = %q, want %q", ui.Error, "connection refused")
	}
}

func TestNodeService_Normalize_Valid(t *testing.T) {
	s := &NodeService{}
	n := &model.Node{
		Name:     "  primary  ",
		ApiToken: "  abc  ",
		Address:  "example.com",
		Port:     8443,
		Scheme:   "",
		BasePath: "panel",
	}
	if err := s.normalize(n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Name != "primary" {
		t.Fatalf("Name not trimmed: %q", n.Name)
	}
	if n.ApiToken != "abc" {
		t.Fatalf("ApiToken not trimmed: %q", n.ApiToken)
	}
	if n.Scheme != "https" {
		t.Fatalf("empty Scheme should default to https, got %q", n.Scheme)
	}
	if n.BasePath != "/panel/" {
		t.Fatalf("BasePath = %q, want /panel/", n.BasePath)
	}
}

func TestNodeService_Normalize_KeepsValidScheme(t *testing.T) {
	s := &NodeService{}
	n := &model.Node{Name: "n", Address: "example.com", Port: 80, Scheme: "http"}
	if err := s.normalize(n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Scheme != "http" {
		t.Fatalf("Scheme = %q, want http", n.Scheme)
	}
}

func TestNodeService_Normalize_RejectsEmptyName(t *testing.T) {
	s := &NodeService{}
	n := &model.Node{Name: "   ", Address: "example.com", Port: 443}
	if err := s.normalize(n); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestNodeService_Normalize_RejectsBadHost(t *testing.T) {
	s := &NodeService{}
	n := &model.Node{Name: "n", Address: "bad host name with spaces", Port: 443}
	if err := s.normalize(n); err == nil {
		t.Fatal("expected error for invalid host")
	}
}

func TestNodeService_Normalize_RejectsOutOfRangePort(t *testing.T) {
	s := &NodeService{}
	for _, port := range []int{0, -1, 65536, 100000} {
		n := &model.Node{Name: "n", Address: "example.com", Port: port}
		if err := s.normalize(n); err == nil {
			t.Fatalf("expected error for port %d", port)
		}
	}
}

func TestNodeService_Normalize_OverridesUnknownScheme(t *testing.T) {
	s := &NodeService{}
	n := &model.Node{Name: "n", Address: "example.com", Port: 443, Scheme: "ftp"}
	if err := s.normalize(n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Scheme != "https" {
		t.Fatalf("Scheme = %q, want https", n.Scheme)
	}
}
