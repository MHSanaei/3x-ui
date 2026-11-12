package vkturn

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInstanceUsesWireGuardListenAddress(t *testing.T) {
	for _, tc := range []struct{ listen, want string }{
		{"", "127.0.0.1:51820"}, {"0.0.0.0", "127.0.0.1:51820"}, {"::", "[::1]:51820"}, {"192.0.2.1", "192.0.2.1:51820"},
	} {
		t.Run(tc.listen, func(t *testing.T) {
			inbound := &model.Inbound{Id: 4, Enable: true, Protocol: model.WireGuard, Port: 51820, Listen: tc.listen, Settings: `{"vkTurn":{"enabled":true,"callLink":"https://vk.ru/call/join/example","port":56000}}`}
			inst, ok := InstanceFromInbound(inbound)
			if !ok || inst.Connect != tc.want || inst.Listen != "0.0.0.0:56000" {
				t.Fatalf("instance = %+v, valid = %v", inst, ok)
			}
		})
	}
}

func TestSettingsRejectInvalidEnabledTunnel(t *testing.T) {
	for _, tc := range []struct{ settings, want string }{
		{`{"enabled":true,"callLink":"https://example.com/call/join/token","port":56000}`, "VK call link must be an HTTPS vk.ru or vk.com call/join URL"},
		{`{"enabled":true,"callLink":"https://vk.ru/call/join/token","port":0}`, "VK TURN UDP port must be between 1 and 65535"},
		{`{"enabled":true,"callLink":"https://vk.ru/call/join/token","port":51820}`, "VK TURN UDP port must differ from the WireGuard port"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			_, err := SettingsFromInbound(&model.Inbound{Protocol: model.WireGuard, Port: 51820, Settings: `{"vkTurn":` + tc.settings + `}`})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLegacyAndDisabledWireGuardDoNotStartTunnel(t *testing.T) {
	for _, settings := range []string{`{}`, `{"vkTurn":{"enabled":false}}`} {
		inbound := &model.Inbound{Enable: true, Protocol: model.WireGuard, Settings: settings}
		if _, err := SettingsFromInbound(inbound); err != nil {
			t.Fatal(err)
		}
		if inst, ok := InstanceFromInbound(inbound); ok {
			t.Fatalf("unexpected tunnel: %+v", inst)
		}
	}
}
