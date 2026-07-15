package naive

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestToConfig(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())
	outbound := &model.NaiveOutbound{
		Tag:                 "naive-a",
		ProxyURL:            "https://user:pass@example.com:443",
		LocalPort:           30123,
		Enabled:             true,
		InsecureConcurrency: 3,
		TunnelTimeout:       1800,
		IdleTimeout:         600,
		ExtraHeaders:        "X-Test: value",
		HostResolverRules:   "MAP example.com 1.2.3.4",
		ResolverRange:       "100.64.0.0/10",
		NoPostQuantum:       true,
	}

	cfg := ToConfig(outbound)
	if cfg.Listen != "socks://127.0.0.1:30123" {
		t.Fatalf("listen = %q", cfg.Listen)
	}
	if cfg.Proxy != outbound.ProxyURL {
		t.Fatalf("proxy = %q", cfg.Proxy)
	}
	if cfg.Log != filepath.Join(config.GetLogFolder(), "naive-naive-a.log") {
		t.Fatalf("log = %q", cfg.Log)
	}
	if cfg.ExtraHeaders != outbound.ExtraHeaders {
		t.Fatalf("extra headers = %q", cfg.ExtraHeaders)
	}
	if !cfg.NoPostQuantum {
		t.Fatal("expected no-post-quantum true")
	}
}

func TestConfigOmitsZeroValues(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())
	outbound := &model.NaiveOutbound{
		Tag:       "naive-b",
		ProxyURL:  "https://user:pass@example.com",
		LocalPort: 30001,
	}

	data, err := json.Marshal(ToConfig(outbound))
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"insecure-concurrency",
		"tunnel-timeout",
		"idle-timeout",
		"extra-headers",
		"host-resolver-rules",
		"resolver-range",
		"no-post-quantum",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unexpected key %q in %s", forbidden, text)
		}
	}
}
