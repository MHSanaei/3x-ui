package database

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const wgTestKeys = `"secretKey":"yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=","peers":[{"publicKey":"xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=","endpoint":"engage.cloudflareclient.com:2408"}]`

func TestRewriteWireguardDomainStrategy(t *testing.T) {
	tests := []struct {
		name        string
		outbound    string
		wantChanged bool
		wantRoot    string
		wantSockopt string
		wantDNS     bool
	}{
		{
			name:        "the WARP default moves to both places the core now reads",
			outbound:    `{"protocol":"wireguard","tag":"warp","settings":{"domainStrategy":"ForceIPv4v6",` + wgTestKeys + `}}`,
			wantChanged: true, wantRoot: "ForceIPv4v6", wantSockopt: "ForceIPv4v6",
		},
		{
			name:        "values the admin already set win over the legacy key",
			outbound:    `{"protocol":"wireguard","tag":"wg","targetStrategy":"UseIPv6","streamSettings":{"sockopt":{"domainStrategy":"UseIPv4"}},"settings":{"domainStrategy":"forceipv6",` + wgTestKeys + `}}`,
			wantChanged: true, wantRoot: "UseIPv6", wantSockopt: "UseIPv4",
		},
		{
			name:        "plain ForceIP had no family preference, so only the key goes",
			outbound:    `{"protocol":"wireguard","tag":"wg","settings":{"domainStrategy":"ForceIP",` + wgTestKeys + `}}`,
			wantChanged: true,
		},
		{
			name:        "remoteDNS local becomes targetStrategy, which resolves with the built-in DNS",
			outbound:    `{"protocol":"wireguard","tag":"wg","settings":{"domainStrategy":"ForceIPv4","remoteDNS":["local"],` + wgTestKeys + `}}`,
			wantChanged: true, wantRoot: "ForceIPv4", wantSockopt: "ForceIPv4",
		},
		{
			name:        "remoteDNS local without a strategy resolves any family",
			outbound:    `{"protocol":"wireguard","tag":"wg","settings":{"remoteDNS":["local"],` + wgTestKeys + `}}`,
			wantChanged: true, wantRoot: "ForceIP",
		},
		{
			name:        "a strategy the old core refused is dropped rather than moved",
			outbound:    `{"protocol":"wireguard","tag":"wg","settings":{"domainStrategy":"UseIPv4",` + wgTestKeys + `}}`,
			wantChanged: true,
		},
		{
			name:        "IP remoteDNS entries stay",
			outbound:    `{"protocol":"wireguard","tag":"wg","settings":{"remoteDNS":["1.1.1.1"],` + wgTestKeys + `}}`,
			wantChanged: false, wantDNS: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated, changed, err := rewriteWireguardDomainStrategy(`{"outbounds":[` + tc.outbound + `]}`)
			if err != nil {
				t.Fatalf("rewrite: %v", err)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			var cfg struct {
				Outbounds []json.RawMessage `json:"outbounds"`
			}
			if err := json.Unmarshal([]byte(updated), &cfg); err != nil || len(cfg.Outbounds) != 1 {
				t.Fatalf("rewritten template unreadable (%v): %s", err, updated)
			}
			var got struct {
				TargetStrategy string `json:"targetStrategy"`
				StreamSettings struct {
					Sockopt struct {
						DomainStrategy string `json:"domainStrategy"`
					} `json:"sockopt"`
				} `json:"streamSettings"`
				Settings map[string]any `json:"settings"`
			}
			if err := json.Unmarshal(cfg.Outbounds[0], &got); err != nil {
				t.Fatal(err)
			}
			if got.TargetStrategy != tc.wantRoot {
				t.Errorf("targetStrategy = %q, want %q", got.TargetStrategy, tc.wantRoot)
			}
			if got.StreamSettings.Sockopt.DomainStrategy != tc.wantSockopt {
				t.Errorf("sockopt.domainStrategy = %q, want %q", got.StreamSettings.Sockopt.DomainStrategy, tc.wantSockopt)
			}
			if _, kept := got.Settings["domainStrategy"]; kept {
				t.Errorf("settings.domainStrategy survived the rewrite: %s", cfg.Outbounds[0])
			}
			if _, kept := got.Settings["remoteDNS"]; kept != tc.wantDNS {
				t.Errorf("remoteDNS kept = %v, want %v", kept, tc.wantDNS)
			}
			if err := xray.ValidateOutboundConfig(cfg.Outbounds[0]); err != nil {
				t.Fatalf("xray-core refuses the rewritten outbound: %v", err)
			}
		})
	}
}

func TestWireguardDomainStrategySeederRewritesStoredTemplateOnce(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := InitDB(config.GetDBPath()); err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skipf("sqlite needs cgo: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	legacy := `{"outbounds":[{"protocol":"wireguard","tag":"warp","settings":{"domainStrategy":"ForceIPv4v6",` + wgTestKeys + `}}]}`
	seedTemplate(t, legacy)
	if err := db.Where("seeder_name = ?", "WireguardDomainStrategyFix").
		Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}

	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}
	var cfg struct {
		Outbounds []struct {
			TargetStrategy string         `json:"targetStrategy"`
			Settings       map[string]any `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(storedTemplate(t)), &cfg); err != nil || len(cfg.Outbounds) != 1 {
		t.Fatalf("stored template unreadable (%v)", err)
	}
	if _, kept := cfg.Outbounds[0].Settings["domainStrategy"]; kept || cfg.Outbounds[0].TargetStrategy != "ForceIPv4v6" {
		t.Fatalf("stored outbound was not rewritten: %+v", cfg.Outbounds[0])
	}

	// The history gate keeps a hand-edited template from being rewritten on every restart.
	seedTemplate(t, legacy)
	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}
	if got := storedTemplate(t); got != legacy {
		t.Errorf("a completed seeder rewrote the template again: %s", got)
	}
}
