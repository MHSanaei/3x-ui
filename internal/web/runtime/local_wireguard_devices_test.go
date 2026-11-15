package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"testing"

	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/proxy/wireguard"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type wireguardDeviceServer struct {
	command.UnimplementedHandlerServiceServer
	users chan *protocol.User
}

func (s *wireguardDeviceServer) AlterInbound(_ context.Context, request *command.AlterInboundRequest) (*command.AlterInboundResponse, error) {
	operation := new(command.AddUserOperation)
	if err := proto.Unmarshal(request.Operation.Value, operation); err != nil {
		return nil, err
	}
	s.users <- operation.User
	return &command.AlterInboundResponse{}, nil
}

func newWireguardDeviceRuntime(t *testing.T) (*Local, *wireguardDeviceServer) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	core := &wireguardDeviceServer{users: make(chan *protocol.User, 3)}
	server := grpc.NewServer()
	command.RegisterHandlerServiceServer(server, core)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	port := listener.Addr().(*net.TCPAddr).Port
	local := NewLocal(LocalDeps{APIPort: func() int { return port }})
	return local, core
}

func wireguardDeviceClient(keepAlive *model.KeepAliveValue) model.Client {
	key := func(seed byte) string {
		return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{seed}, 32))
	}
	return model.Client{
		Email: "owner", Enable: true, PublicKey: key(1), AllowedIPs: []string{"10.0.0.2/32"},
		KeepAlive: keepAlive,
		WireguardDevices: []model.WireguardDevice{
			{Id: "phone", Enable: true, PublicKey: key(2), AllowedIPs: []string{"10.0.0.3/32"}},
			{Id: "off", Enable: false, PublicKey: key(3), AllowedIPs: []string{"10.0.0.4/32"}},
		},
	}
}

func assertWireguardDevicePeers(t *testing.T, core *wireguardDeviceServer, keepAlive string) {
	t.Helper()
	if len(core.users) != 2 {
		t.Fatalf("added %d peers, want primary and enabled phone", len(core.users))
	}
	for i := range 2 {
		user := <-core.users
		if user.Email != "owner" {
			t.Fatalf("peer attribution = %q", user.Email)
		}
		peer := new(wireguard.PeerConfig)
		if err := proto.Unmarshal(user.Account.Value, peer); err != nil {
			t.Fatal(err)
		}
		if peer.KeepAlive != keepAlive || peer.AllowedIps[0] != []string{"10.0.0.2/32", "10.0.0.3/32"}[i] {
			t.Fatalf("unexpected peer %v", peer)
		}
	}
}

func TestLocalWireguardAddsEveryEnabledDevice(t *testing.T) {
	for _, test := range []struct {
		name      string
		keepAlive *model.KeepAliveValue
		want      string
	}{
		{name: "omitted"},
		{name: "zero", keepAlive: model.KeepAlivePtr(0)},
		{name: "positive", keepAlive: model.KeepAlivePtr(25), want: "25"},
	} {
		t.Run(test.name, func(t *testing.T) {
			local, core := newWireguardDeviceRuntime(t)
			if err := local.AddClient(context.Background(), &model.Inbound{Tag: "wg", Protocol: model.WireGuard}, wireguardDeviceClient(test.keepAlive)); err != nil {
				t.Fatal(err)
			}
			assertWireguardDevicePeers(t, core, test.want)
		})
	}
}

func TestLocalWireguardAddUserAcceptsLegacyKeepAliveStrings(t *testing.T) {
	for _, keepAlive := range []string{"", "0", "25"} {
		t.Run("keepAlive="+keepAlive, func(t *testing.T) {
			local, core := newWireguardDeviceRuntime(t)
			client := wireguardDeviceClient(nil)
			user := map[string]any{
				"email":            client.Email,
				"publicKey":        client.PublicKey,
				"allowedIPs":       client.AllowedIPs,
				"wireguardDevices": client.WireguardDevices,
				"keepAlive":        keepAlive,
			}
			if err := local.AddUser(context.Background(), &model.Inbound{Tag: "wg", Protocol: model.WireGuard}, user); err != nil {
				t.Fatal(err)
			}
			want := keepAlive
			if keepAlive == "0" {
				want = ""
			}
			assertWireguardDevicePeers(t, core, want)
			if user["keepAlive"] != keepAlive {
				t.Fatal("AddUser changed the caller's keepAlive value")
			}
		})
	}
}
