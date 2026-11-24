package service

import (
	"maps"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/proxy/wireguard"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestWireguardDeviceChangesKeepOtherCorePeers(t *testing.T) {
	newKey := func() (string, string) {
		t.Helper()
		private, public, err := wgutil.GenerateWireguardKeypair()
		if err != nil {
			t.Fatal(err)
		}
		return private, public
	}
	serverKey, _ := newKey()
	_, primary := newKey()
	_, phone := newKey()
	_, other := newKey()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := udp.LocalAddr().(*net.UDPAddr).Port
	_ = udp.Close()
	inbound := model.Inbound{Tag: "wg-hot", Listen: "127.0.0.1", Port: port, Protocol: model.WireGuard}
	clients := []model.Client{
		{Email: "owner", Enable: true, PublicKey: primary, AllowedIPs: []string{"10.0.0.2/32"}, KeepAlive: model.KeepAlivePtr(25)},
		{Email: "other", Enable: true, PublicKey: other, AllowedIPs: []string{"10.0.0.3/32"}},
	}
	render := func() *xray.InboundConfig {
		inbound.Settings = string(mustJSON(t, map[string]any{
			"secretKey": serverKey, "address": []string{"10.0.0.1/24"}, "clients": clients,
		}))
		return inbound.GenXrayInboundConfig()
	}
	initial := isolationConfig(t, *render())
	api := startIsolationCore(t, initial)

	device := model.WireguardDevice{
		Id: "phone", Enable: true, PublicKey: phone, AllowedIPs: []string{"10.0.0.4/32"},
	}
	disabled := device
	disabled.Enable = false
	previous := initial
	for _, step := range []struct {
		name    string
		devices []model.WireguardDevice
		active  bool
	}{
		{name: "add", devices: []model.WireguardDevice{device}, active: true},
		{name: "disable", devices: []model.WireguardDevice{disabled}},
		{name: "enable", devices: []model.WireguardDevice{device}, active: true},
		{name: "delete"},
	} {
		clients[0].WireguardDevices = step.devices
		next := *previous
		next.InboundConfigs = slices.Clone(previous.InboundConfigs)
		next.InboundConfigs[len(next.InboundConfigs)-1] = *render()
		diff, ok := xray.ComputeHotDiff(previous, &next)
		if !ok || diff.DropsUsers() || len(diff.RemovedInboundTags)+len(diff.AddedInbounds) != 0 ||
			len(diff.AddedUsers) != 1 || diff.AddedUsers[0].Email != "owner" ||
			diff.AddedUsers[0].User["keepAlive"] != "25" {
			t.Fatalf("%s: expected only owner peer changes, got %+v", step.name, diff)
		}
		if step.active && len(diff.RemovedUsers) != 0 ||
			!step.active && (len(diff.RemovedUsers) != 1 || diff.RemovedUsers[0].Email != "owner") {
			t.Fatalf("%s: unexpected removals: %+v", step.name, diff.RemovedUsers)
		}
		if !applyXrayHotDiff(api, diff) {
			t.Fatalf("%s: peer update failed", step.name)
		}
		users, err := (*api.HandlerServiceClient).GetInboundUsers(t.Context(), &command.GetInboundUserRequest{Tag: inbound.Tag})
		if err != nil {
			t.Fatal(err)
		}
		got := make(map[string]string)
		for _, user := range users.GetUsers() {
			account, err := user.GetAccount().GetInstance()
			if err != nil {
				t.Fatal(err)
			}
			got[account.(*wireguard.PeerConfig).GetPublicKey()] = user.GetEmail()
		}
		primaryHex, _ := wgutil.KeyToHex(primary)
		otherHex, _ := wgutil.KeyToHex(other)
		phoneHex, _ := wgutil.KeyToHex(phone)
		want := map[string]string{primaryHex: "owner", otherHex: "other"}
		if step.active {
			want[phoneHex] = "owner"
		}
		if !maps.Equal(got, want) {
			t.Fatalf("%s: core peers = %v, want %v", step.name, got, want)
		}
		previous = &next
	}
}

func TestNodeWireguardDevicePushSchedulesReconcile(t *testing.T) {
	setupConflictDB(t)
	wasPending := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() { isNeedXrayReconcile.Store(wasPending) })
	probe := &inboundOpsProbe{}
	useTestRuntimeManager(t).SetLocalRuntimeOverride(probe)

	client := model.Client{
		Email: "wg-owner", Enable: true, PublicKey: strings.Repeat("01", 32),
		AllowedIPs: []string{"10.0.0.2/32"},
	}
	seedInboundConflict(t, "wg-node", "127.0.0.1", 50091, model.WireGuard, "", clientsSettings(t, []model.Client{client}))
	stored := loadInboundByTag(t, "wg-node")
	if err := (&ClientService{}).SyncInbound(nil, stored.Id, []model.Client{client}); err != nil {
		t.Fatal(err)
	}
	mkTraffic(t, stored.Id, client.Email, 0, 0, 0, 0, true)

	client.WireguardDevices = []model.WireguardDevice{{
		Id: "phone", Enable: true, PublicKey: strings.Repeat("02", 32), AllowedIPs: []string{"10.0.0.3/32"},
	}}
	pushed := *stored
	pushed.Settings = clientsSettings(t, []model.Client{client})
	if _, restart, err := (&InboundService{FromNodeSync: true}).UpdateInbound(&pushed); err != nil || restart {
		t.Fatalf("device push returned restart=%t, err=%v", restart, err)
	}
	if ops := probe.recorded(); len(ops) != 0 || !isNeedXrayReconcile.Load() {
		t.Fatalf("expected scheduled reconciliation, got runtime ops %q", ops)
	}
}
