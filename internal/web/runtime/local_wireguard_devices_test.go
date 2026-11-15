package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/proxy/wireguard"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
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

func TestLocalWireguardAddsEveryEnabledDevice(t *testing.T) {
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
	key := func(seed byte) string {
		return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{seed}, 32))
	}
	client := model.Client{
		Email: "owner", Enable: true, PublicKey: key(1), AllowedIPs: []string{"10.0.0.2/32"},
		KeepAlive: model.KeepAlivePtr(25),
		WireguardDevices: []model.WireguardDevice{
			{Id: "phone", Enable: true, PublicKey: key(2), AllowedIPs: []string{"10.0.0.3/32"}},
			{Id: "off", Enable: false, PublicKey: key(3), AllowedIPs: []string{"10.0.0.4/32"}},
		},
	}
	if err := local.AddClient(context.Background(), &model.Inbound{Tag: "wg", Protocol: model.WireGuard}, client); err != nil {
		t.Fatal(err)
	}
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
		if peer.KeepAlive != "25" || peer.AllowedIps[0] != []string{"10.0.0.2/32", "10.0.0.3/32"}[i] {
			t.Fatalf("unexpected peer %v", peer)
		}
	}
}
