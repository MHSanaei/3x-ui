package runtime

import (
	"context"
	"encoding/base64"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/wireguard"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type clientAccountCore struct {
	command.UnimplementedHandlerServiceServer
	mu       sync.Mutex
	users    []*protocol.User
	accounts []*serial.TypedMessage
	removed  []string
}

func (c *clientAccountCore) GetInboundUsers(context.Context, *command.GetInboundUserRequest) (*command.GetInboundUserResponse, error) {
	return &command.GetInboundUserResponse{Users: c.users}, nil
}

func (c *clientAccountCore) AlterInbound(_ context.Context, request *command.AlterInboundRequest) (*command.AlterInboundResponse, error) {
	switch {
	case strings.HasSuffix(request.GetOperation().GetType(), "AddUserOperation"):
		operation := &command.AddUserOperation{}
		if err := proto.Unmarshal(request.Operation.Value, operation); err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.accounts = append(c.accounts, proto.Clone(operation.User.Account).(*serial.TypedMessage))
		c.mu.Unlock()
	case strings.HasSuffix(request.GetOperation().GetType(), "RemoveUserOperation"):
		operation := &command.RemoveUserOperation{}
		if err := proto.Unmarshal(request.Operation.Value, operation); err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.removed = append(c.removed, operation.Email)
		c.mu.Unlock()
	}
	return &command.AlterInboundResponse{}, nil
}

func TestLocalClientOperationsPreserveProtocolAccounts(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	const email = "renew@example.test"
	for _, action := range []string{"add", "update"} {
		for _, name := range []string{"vless", "shadowsocks", "shadowsocks2022", "wireguard"} {
			t.Run(action+"/"+name, func(t *testing.T) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				server := grpc.NewServer()
				core := &clientAccountCore{
					users: []*protocol.User{{Email: email}, {Email: "other@example.test"}, {Email: email}},
				}
				command.RegisterHandlerServiceServer(server, core)
				go func() { _ = server.Serve(listener) }()
				t.Cleanup(server.Stop)
				local := NewLocal(LocalDeps{APIPort: func() int { return listener.Addr().(*net.TCPAddr).Port }})
				client := model.Client{Email: email, ID: id, Password: "keep-password", Enable: true, Reverse: &model.ClientReverse{Tag: "portal"}}
				inbound := &model.Inbound{Tag: "renew", Protocol: model.Protocol(name), Settings: `{"clients":[]}`}
				if name == "wireguard" {
					client = wireguardDeviceClient(model.KeepAlivePtr(25))
					client.Email = email
				}
				if name == "shadowsocks" {
					inbound.Settings = `{"clients":[],"method":"aes-128-gcm"}`
				}
				if name == "shadowsocks2022" {
					inbound.Protocol = model.Shadowsocks
					inbound.Settings = `{"clients":[],"method":"2022-blake3-aes-128-gcm"}`
					client.Password = base64.StdEncoding.EncodeToString([]byte("0123456789012345"))
				}
				if action == "add" {
					err = local.AddClient(context.Background(), inbound, client)
				} else {
					err = local.UpdateUser(context.Background(), inbound, client.Email, client)
				}
				if err != nil {
					t.Fatal(err)
				}
				core.mu.Lock()
				defer core.mu.Unlock()
				wantAccounts := 1
				if name == "wireguard" {
					wantAccounts = 2
				}
				if len(core.accounts) != wantAccounts {
					t.Fatalf("core received %d accounts, want %d", len(core.accounts), wantAccounts)
				}
				account := core.accounts[0]
				switch name {
				case "vless":
					decoded := &vless.Account{}
					if err := proto.Unmarshal(account.Value, decoded); err != nil {
						t.Fatal(err)
					}
					if decoded.Id != id || decoded.GetReverse().GetTag() != "portal" {
						t.Fatalf("VLESS identity/reverse dropped: %+v", decoded)
					}
				case "shadowsocks":
					decoded := &shadowsocks.Account{}
					if err := proto.Unmarshal(account.Value, decoded); err != nil {
						t.Fatal(err)
					}
					if decoded.Password != client.Password || decoded.CipherType != shadowsocks.CipherType_AES_128_GCM {
						t.Fatalf("Shadowsocks account changed: %+v", decoded)
					}
				case "shadowsocks2022":
					if !strings.Contains(account.Type, "shadowsocks_2022") {
						t.Fatalf("wrong Shadowsocks account type: %s", account.Type)
					}
					decoded := &shadowsocks_2022.Account{}
					if err := proto.Unmarshal(account.Value, decoded); err != nil {
						t.Fatal(err)
					}
					if decoded.Key != client.Password {
						t.Fatalf("Shadowsocks 2022 key changed: %+v", decoded)
					}
				case "wireguard":
					var wantRemoved []string
					if action == "update" {
						wantRemoved = []string{email, email}
					}
					if !slices.Equal(core.removed, wantRemoved) {
						t.Fatalf("removed WireGuard peers %v, want %v", core.removed, wantRemoved)
					}
					for i, wantIPs := range [][]string{client.AllowedIPs, client.WireguardDevices[0].AllowedIPs} {
						decoded := &wireguard.PeerConfig{}
						if err := proto.Unmarshal(core.accounts[i].Value, decoded); err != nil {
							t.Fatal(err)
						}
						if decoded.KeepAlive != "25" || !slices.Equal(decoded.AllowedIps, wantIPs) {
							t.Fatalf("WireGuard peer changed: %+v", decoded)
						}
					}
				}
			})
		}
	}
}
