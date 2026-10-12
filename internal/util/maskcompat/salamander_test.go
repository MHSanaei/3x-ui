package maskcompat

import (
	"context"
	"encoding/json"
	"testing"

	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/transport/internet"
)

// coreListensSalamander opens a listener through the finalmask the way an inbound does; the core
// checks the salamander password only here, never when it loads the config.
func coreListensSalamander(t *testing.T, password string) bool {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"finalmask": map[string]any{"udp": []any{
		map[string]any{"type": "salamander", "settings": map[string]any{"password": password}},
	}}})
	var sc conf.StreamConfig
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatalf("decode stream: %v", err)
	}
	built, err := sc.Build()
	if err != nil {
		t.Fatalf("the core refuses to load the stream: %v", err)
	}
	mss, err := internet.ToMemoryStreamConfig(built)
	if err != nil {
		t.Fatalf("memory stream: %v", err)
	}
	conn, err := mss.FinalMask.ListenPacket(context.Background(), &xnet.UDPAddr{IP: []byte{127, 0, 0, 1}})
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// The panel must refuse exactly the passwords the core refuses: it counts bytes, not characters.
func TestCheckSalamanderPasswordsMatchesTheCore(t *testing.T) {
	for _, password := range []string{"", "abc", "€", "abcd", "a€", "long-enough"} {
		t.Run(password, func(t *testing.T) {
			finalmask := map[string]any{"udp": []any{
				map[string]any{"type": "salamander", "settings": map[string]any{"password": password}},
			}}
			panelAccepts := CheckSalamanderPasswords(finalmask) == nil
			if coreAccepts := coreListensSalamander(t, password); panelAccepts != coreAccepts {
				t.Fatalf("password %q: panel accepts %v, core listens %v", password, panelAccepts, coreAccepts)
			}
		})
	}
}
