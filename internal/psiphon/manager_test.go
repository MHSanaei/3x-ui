package psiphon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCountryFlag(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"US", "🇺🇸"},
		{"de", "🇩🇪"},
		{"JP", "🇯🇵"},
		{"", "🌐"},
		{"INVALID", "🌐"},
	}
	for _, tt := range tests {
		got := CountryFlag(tt.code)
		if got != tt.want {
			t.Errorf("CountryFlag(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestCountryName(t *testing.T) {
	if got := CountryName("US"); got != "United States" {
		t.Fatalf("CountryName(US) = %q, want 'United States'", got)
	}
	if got := CountryName(""); got != "Best / Auto" {
		t.Fatalf("CountryName('') = %q, want 'Best / Auto'", got)
	}
}

func TestManagerPortAllocation(t *testing.T) {
	mgr := NewManager()

	port1, err := mgr.AllocatePort(0)
	if err != nil {
		t.Fatalf("AllocatePort failed: %v", err)
	}
	if port1 < DefaultBasePort || port1 > DefaultMaxPort {
		t.Fatalf("port %d out of range", port1)
	}

	inst1 := &Instance{
		ID:          "US",
		CountryCode: "US",
		CountryName: "United States",
		SocksPort:   port1,
		Status:      StatusStopped,
		Enabled:     true,
	}
	if err := mgr.RegisterInstance(inst1); err != nil {
		t.Fatalf("RegisterInstance failed: %v", err)
	}

	port2, err := mgr.AllocatePort(port1)
	if err != nil {
		t.Fatalf("AllocatePort(port1) failed: %v", err)
	}
	if port2 == port1 {
		t.Fatalf("port %d should not be allocated twice", port2)
	}

	mgr.UnregisterInstance("US")
	if _, ok := mgr.GetInstance("US"); ok {
		t.Fatalf("instance US should have been unregistered")
	}
}

func TestWriteInstanceConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.json")
	baseJSON := `{"PropagationChannelId":"TEST1234","SponsorId":"SPON1234","LocalSocksProxyPort":1234}`

	err := writeInstanceConfigFile(cfgPath, tempDir, "DE", 40010, baseJSON)
	if err != nil {
		t.Fatalf("writeInstanceConfigFile failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config file failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}

	if parsed["PropagationChannelId"] != "TEST1234" {
		t.Errorf("PropagationChannelId = %v, want TEST1234", parsed["PropagationChannelId"])
	}
	// Protected overrides must win over pasted JSON
	if parsed["LocalSocksProxyPort"] != float64(40010) {
		t.Errorf("LocalSocksProxyPort = %v, want 40010", parsed["LocalSocksProxyPort"])
	}
	if parsed["EgressRegion"] != "DE" {
		t.Errorf("EgressRegion = %v, want DE", parsed["EgressRegion"])
	}
	if parsed["ListenInterface"] != "lo" {
		t.Errorf("ListenInterface = %v, want lo", parsed["ListenInterface"])
	}
	if parsed["DisableLocalSocksProxy"] != false {
		t.Errorf("DisableLocalSocksProxy = %v, want false", parsed["DisableLocalSocksProxy"])
	}
	if parsed["DataRootDirectory"] != tempDir {
		t.Errorf("DataRootDirectory = %v, want %s", parsed["DataRootDirectory"], tempDir)
	}
}

func TestNoticeHandling(t *testing.T) {
	mgr := NewManager()
	inst := &Instance{
		ID:          "NL",
		CountryCode: "NL",
		CountryName: "Netherlands",
		SocksPort:   40020,
		Status:      StatusStopped,
		Enabled:     true,
	}
	_ = mgr.RegisterInstance(inst)

	// Simulate notices
	mgr.handleNotice("NL", Notice{
		NoticeType: "ListeningSocksProxyPort",
		Data:       map[string]any{"port": 40020},
	})
	snap, _ := mgr.GetInstance("NL")
	if snap.Status != StatusStarting {
		t.Errorf("status = %q, want starting", snap.Status)
	}

	mgr.handleNotice("NL", Notice{
		NoticeType: "ActiveTunnel",
		Data:       map[string]any{"count": 1},
	})
	snap, _ = mgr.GetInstance("NL")
	if snap.Status != StatusConnected {
		t.Errorf("status = %q, want connected", snap.Status)
	}

	mgr.handleNotice("NL", Notice{
		NoticeType: "AvailableEgressRegions",
		Data:       map[string]any{"regions": []any{"US", "DE", "ZZ"}},
	})
	countries := mgr.GetAvailableCountries()
	hasZZ := false
	for _, c := range countries {
		if c.Code == "ZZ" {
			hasZZ = true
			break
		}
	}
	if !hasZZ {
		t.Errorf("expected dynamic country ZZ to be available")
	}
}
