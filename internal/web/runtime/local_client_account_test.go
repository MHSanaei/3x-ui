package runtime

import (
	"context"
	"encoding/base64"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/xtls/xray-core/app/proxyman/command"
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
	accounts []*serial.TypedMessage
}

func (c *clientAccountCore) AlterInbound(_ context.Context, request *command.AlterInboundRequest) (*command.AlterInboundResponse, error) {
	if strings.HasSuffix(request.GetOperation().GetType(), "AddUserOperation") {
		operation := &command.AddUserOperation{}
		if err := proto.Unmarshal(request.Operation.Value, operation); err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.accounts = append(c.accounts, proto.Clone(operation.User.Account).(*serial.TypedMessage))
		c.mu.Unlock()
	}
	return &command.AlterInboundResponse{}, nil
}

func TestLocalClientOperationsPreserveProtocolAccounts(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	for _, action := range []string{"add", "update"} {
		for _, protocol := range []string{"vless", "shadowsocks", "shadowsocks2022", "wireguard"} {
			t.Run(action+"/"+protocol, func(t *testing.T) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				server := grpc.NewServer()
				core := &clientAccountCore{}
				command.RegisterHandlerServiceServer(server, core)
				go func() { _ = server.Serve(listener) }()
				t.Cleanup(server.Stop)
				local := NewLocal(LocalDeps{APIPort: func() int { return listener.Addr().(*net.TCPAddr).Port }})
				client := model.Client{Email: "renew@example.test", ID: id, Password: "keep-password", Enable: true, Reverse: &model.ClientReverse{Tag: "portal"}, PublicKey: key, AllowedIPs: []string{"10.0.0.2/32"}, KeepAlive: model.KeepAlivePtr(25)}
				inbound := &model.Inbound{Tag: "renew", Protocol: model.Protocol(protocol), Settings: `{"clients":[]}`}
				if protocol == "shadowsocks" {
					inbound.Settings = `{"clients":[],"method":"aes-128-gcm"}`
				}
				if protocol == "shadowsocks2022" {
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
				if len(core.accounts) != 1 {
					t.Fatalf("core received %d accounts", len(core.accounts))
				}
				account := core.accounts[0]
				switch protocol {
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
					decoded := &wireguard.PeerConfig{}
					if err := proto.Unmarshal(account.Value, decoded); err != nil {
						t.Fatal(err)
					}
					if decoded.KeepAlive != "25" || len(decoded.AllowedIps) != 1 || decoded.AllowedIps[0] != "10.0.0.2/32" {
						t.Fatalf("WireGuard peer changed: %+v", decoded)
					}
				}
			})
		}
	}
}
