package xray

import (
	"context"
	"errors"
	"testing"

	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type wireguardUsersClient struct {
	command.HandlerServiceClient
	users     []*protocol.User
	removed   []string
	listErr   error
	removeErr error
}

func (c *wireguardUsersClient) GetInboundUsers(_ context.Context, request *command.GetInboundUserRequest, _ ...grpc.CallOption) (*command.GetInboundUserResponse, error) {
	if request.Tag != "wg" || request.Email != "" {
		return nil, errors.New("expected every user of the WireGuard inbound")
	}
	return &command.GetInboundUserResponse{Users: c.users}, c.listErr
}

func (c *wireguardUsersClient) AlterInbound(_ context.Context, request *command.AlterInboundRequest, _ ...grpc.CallOption) (*command.AlterInboundResponse, error) {
	operation := new(command.RemoveUserOperation)
	if err := proto.Unmarshal(request.Operation.Value, operation); err != nil {
		return nil, err
	}
	c.removed = append(c.removed, operation.Email)
	return &command.AlterInboundResponse{}, c.removeErr
}

func TestRemoveWireguardUsersRemovesEveryPeerOfClient(t *testing.T) {
	core := &wireguardUsersClient{users: []*protocol.User{
		{Email: "client"}, {Email: "other"}, {Email: "client"}, nil, {Email: "client"},
	}}
	var handler command.HandlerServiceClient = core
	api := XrayAPI{HandlerServiceClient: &handler}
	if err := api.RemoveWireguardUsers("wg", "client"); err != nil {
		t.Fatal(err)
	}
	if len(core.removed) != 3 {
		t.Fatalf("removed %v, want all three client peers", core.removed)
	}
	for _, email := range core.removed {
		if email != "client" {
			t.Fatalf("removed another client's peer: %q", email)
		}
	}
}

func TestRemoveWireguardUsersPropagatesCoreFailures(t *testing.T) {
	for _, failList := range []bool{true, false} {
		core := &wireguardUsersClient{users: []*protocol.User{{Email: "client"}}}
		failure := errors.New("core unavailable")
		if failList {
			core.listErr = failure
		} else {
			core.removeErr = failure
		}
		var handler command.HandlerServiceClient = core
		api := XrayAPI{HandlerServiceClient: &handler}
		if err := api.RemoveWireguardUsers("wg", "client"); !errors.Is(err, failure) {
			t.Fatalf("failure not propagated: %v", err)
		}
	}
}
