package service

import (
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"

	tcplimit "github.com/mhsanaei/3x-ui/v3/internal/tcp-limit"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestTCPConnectionLimitPorts(t *testing.T) {
	cfg := &xray.Config{InboundConfigs: []xray.InboundConfig{
		{Protocol: "vless", Port: 443, Listen: json_util.RawMessage(`"0.0.0.0"`), StreamSettings: json_util.RawMessage(`{"network":"tcp"}`)},
		{Protocol: "trojan", Port: 8443, Listen: json_util.RawMessage(`"::"`), StreamSettings: json_util.RawMessage(`{"network":"grpc"}`)},
		{Protocol: "vmess", Port: 9443, StreamSettings: json_util.RawMessage(`{"network":"ws"}`)},
		{Protocol: "tunnel", Port: 62789, Listen: json_util.RawMessage(`"127.0.0.1"`)},
		{Protocol: "socks", Port: 65102, Listen: json_util.RawMessage(`"::1"`)},
		{Protocol: "wireguard", Port: 80},
		{Protocol: "vless", Port: 8000, StreamSettings: json_util.RawMessage(`{"network":"kcp"}`)},
		{Protocol: "trojan", Port: 8001, StreamSettings: json_util.RawMessage(`{"network":"quic"}`)},
		{Protocol: "vless", Port: 0},
		{Protocol: "vless", Port: 65536},
		{Protocol: "vless", Port: 8002, Listen: json_util.RawMessage(`"/run/xray.sock"`)},
		{Protocol: "vless", Port: 443, Listen: json_util.RawMessage(`"192.0.2.1"`)},
		{Protocol: "trojan", Port: 443, Listen: json_util.RawMessage(`"2001:db8::1"`)},
		{Protocol: "vless", Port: 8080, Listen: json_util.RawMessage(`"::ffff:127.0.0.1"`)},
		{Protocol: "vless", Port: 9444, Listen: json_util.RawMessage(`"::"`), StreamSettings: json_util.RawMessage(`{"network":"tcp","sockopt":{"v6only":true}}`)},
	}}
	ports, err := tcpConnectionLimitTargets(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ports, []tcplimit.Target{{Port: 443}, {Port: 8443}, {Port: 9443}, {Address: netip.MustParseAddr("192.0.2.1"), Port: 443}, {Address: netip.MustParseAddr("2001:db8::1"), Port: 443}, {Address: netip.IPv6Unspecified(), Port: 9444}}) {
		t.Fatalf("ports = %v", ports)
	}
}

type testTCPConnectionLimiter struct {
	limits []int
	err    error
}

func (m *testTCPConnectionLimiter) Apply(limit int, _ []tcplimit.Target) error {
	m.limits = append(m.limits, limit)
	return m.err
}

func (m *testTCPConnectionLimiter) Disable() error { return m.Apply(0, nil) }

func (m *testTCPConnectionLimiter) Stage(limit int, ports []tcplimit.Target) (func() error, error) {
	return func() error { return m.Apply(0, nil) }, m.Apply(limit, ports)
}

func TestTCPConnectionLimitRuntimeMutationRollback(t *testing.T) {
	setupSettingTestDB(t)
	previous := tcpConnectionLimiter
	t.Cleanup(func() { tcpConnectionLimiter = previous })
	s := &SettingService{}
	if err := s.setInt("tcpPerIpConnectionLimit", 200); err != nil {
		t.Fatal(err)
	}
	for _, stageFails := range []bool{false, true} {
		m := &testTCPConnectionLimiter{}
		if stageFails {
			m.err = errors.New("nft failed")
		}
		tcpConnectionLimiter = m
		called := false
		err := WithTCPConnectionLimit(func() error {
			called = true
			if len(m.limits) != 1 || m.limits[0] != 200 {
				t.Fatal("listener changed before protection was applied")
			}
			return errors.New("gRPC apply failed")
		})
		if err == nil {
			t.Fatal("mutation failure was hidden")
		}
		if called == stageFails {
			t.Fatalf("listener mutation called = %v, stage fails = %v", called, stageFails)
		}
		if !stageFails && !slices.Equal(m.limits, []int{200, 0}) {
			t.Fatalf("rollback limits = %v", m.limits)
		}
	}
}

func TestTCPConnectionLimitDisableFailureRetainsSetting(t *testing.T) {
	setupSettingTestDB(t)
	previous := tcpConnectionLimiter
	t.Cleanup(func() { tcpConnectionLimiter = previous })
	s := &SettingService{}
	if err := s.setInt("tcpPerIpConnectionLimit", 200); err != nil {
		t.Fatal(err)
	}
	tcpConnectionLimiter = &testTCPConnectionLimiter{err: errors.New("NET_ADMIN permission lost")}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	settings.TcpPerIpConnectionLimit = 0
	if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil {
		t.Fatal("disable failure was hidden")
	}
	if got, err := s.GetTCPPerIPConnectionLimit(); got != 200 || err != nil {
		t.Fatalf("stored limit = %d, error %v", got, err)
	}
}

func TestTCPConnectionLimitConfigHousekeepingDoesNotReenterLock(t *testing.T) {
	for _, restart := range []bool{false, true} {
		t.Run(map[bool]string{false: "config", true: "restart"}[restart], func(t *testing.T) {
			setupSettingTestDB(t)
			previous := tcpConnectionLimiter
			tcpConnectionLimiter = &testTCPConnectionLimiter{}
			t.Cleanup(func() { tcpConnectionLimiter = previous })
			if err := (&SettingService{}).setInt("tcpPerIpConnectionLimit", 200); err != nil {
				t.Fatal(err)
			}
			ib := &model.Inbound{Tag: "expired", Protocol: model.VLESS, Port: 4443, Enable: true, ExpiryTime: time.Now().Add(-time.Hour).UnixMilli(), Settings: `{"clients":[]}`}
			if err := database.GetDB().Create(ib).Error; err != nil {
				t.Fatal(err)
			}
			calls := 0
			runtime.SetManager(runtime.NewManager(runtime.LocalDeps{
				APIPort: func() int { return 0 },
				WithTCPConnectionLimit: func(apply func() error) error {
					calls++
					return WithTCPConnectionLimit(apply)
				},
			}))
			t.Cleanup(func() { runtime.SetManager(nil) })
			t.Setenv("XUI_BIN_FOLDER", t.TempDir())
			previousProcess := currentXrayProcess()
			t.Cleanup(func() { xrayState.replace(previousProcess) })
			done := make(chan struct{})
			go func() {
				defer close(done)
				if restart {
					_ = (&XrayService{}).RestartXray(true)
				} else {
					_, _ = (&XrayService{}).GetXrayConfig()
				}
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("configuration housekeeping re-entered the reconciliation lock")
			}
			if calls == 0 {
				t.Fatal("expired inbound did not run its protected runtime mutation")
			}
			if err := database.GetDB().First(ib, ib.Id).Error; err != nil || ib.Enable {
				t.Fatalf("expired inbound remains enabled, error %v", err)
			}
		})
	}
}

func TestTCPConnectionLimitSettingApplyFailurePreservesDatabase(t *testing.T) {
	setupSettingTestDB(t)
	previous := tcpConnectionLimiter
	t.Cleanup(func() { tcpConnectionLimiter = previous })
	m := &testTCPConnectionLimiter{err: errors.New("nft permission denied")}
	tcpConnectionLimiter = m
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	settings.TcpPerIpConnectionLimit = 200
	if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil {
		t.Fatal("firewall failure was hidden")
	}
	if got, err := s.GetTCPPerIPConnectionLimit(); err != nil || got != 0 {
		t.Fatalf("saved limit = %d, error %v", got, err)
	}
	if !slices.Equal(m.limits, []int{200}) {
		t.Fatalf("applied limits = %v", m.limits)
	}
}

func TestTCPConnectionLimitSettingDatabaseFailureRollsBackFirewall(t *testing.T) {
	setupSettingTestDB(t)
	previous := tcpConnectionLimiter
	t.Cleanup(func() { tcpConnectionLimiter = previous })
	m := &testTCPConnectionLimiter{}
	tcpConnectionLimiter = m
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	const callback = "test:tcp-limit-save-failure"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			tx.AddError(errors.New("settings save failed"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })
	settings.TcpPerIpConnectionLimit = 200
	if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil {
		t.Fatal("database failure was hidden")
	}
	if got, err := s.GetTCPPerIPConnectionLimit(); err != nil || got != 0 {
		t.Fatalf("saved limit = %d, error %v", got, err)
	}
	if !slices.Equal(m.limits, []int{200, 0}) {
		t.Fatalf("applied limits = %v, want proposed then original", m.limits)
	}
}

func TestTCPConnectionLimitPortsRefusesMalformedConfig(t *testing.T) {
	for _, inbound := range []xray.InboundConfig{
		{Protocol: "vless", Port: 443, Listen: json_util.RawMessage(`{}`)},
		{Protocol: "vless", Port: 443, StreamSettings: json_util.RawMessage(`{`)},
	} {
		if _, err := tcpConnectionLimitTargets(&xray.Config{InboundConfigs: []xray.InboundConfig{inbound}}); err == nil {
			t.Fatal("malformed inbound was accepted")
		}
	}
}

func TestTCPConnectionLimitShadowsocksNetworks(t *testing.T) {
	for _, tc := range []struct {
		settings string
		want     int
	}{
		{`{"network":"udp"}`, 0},
		{`{"network":["udp"]}`, 0},
		{`{"network":"tcp,udp"}`, 1},
		{`{"network":["tcp","udp"]}`, 1},
		{`{"network":"tcp"}`, 1},
		{`{}`, 1},
	} {
		cfg := &xray.Config{InboundConfigs: []xray.InboundConfig{{Protocol: "shadowsocks", Port: 443, Settings: json_util.RawMessage(tc.settings)}}}
		targets, err := tcpConnectionLimitTargets(cfg)
		if err != nil || len(targets) != tc.want {
			t.Errorf("settings %s: targets %v, error %v", tc.settings, targets, err)
		}
	}
}
