package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xtls/xray-core/app/proxyman/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type delayedTuicHandler struct {
	command.UnimplementedHandlerServiceServer
	client  command.HandlerServiceClient
	entered chan struct{}
	resume  chan struct{}
	once    sync.Once
	reject  atomic.Bool
}

func (s *delayedTuicHandler) AddInbound(ctx context.Context, req *command.AddInboundRequest) (*command.AddInboundResponse, error) {
	s.once.Do(func() { close(s.entered) })
	select {
	case <-s.resume:
		if s.reject.Load() {
			return nil, status.Error(codes.Unavailable, "controlled relay add failure")
		}
		return s.client.AddInbound(ctx, req)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *delayedTuicHandler) RemoveInbound(ctx context.Context, req *command.RemoveInboundRequest) (*command.RemoveInboundResponse, error) {
	return s.client.RemoveInbound(ctx, req)
}

func verifyTuicReplacementWindow(t *testing.T, cfg *xray.Config, apiPort int, inbound *model.Inbound, native *tuic.Server, clients []model.Client) {
	t.Helper()
	var previous xray.InboundConfig
	for _, candidate := range cfg.InboundConfigs {
		if candidate.Tag == inbound.Tag {
			previous = candidate
		}
	}
	next := previous
	var settings map[string]any
	if err := json.Unmarshal(previous.Settings, &settings); err != nil {
		t.Fatal(err)
	}
	accounts := settings["accounts"].([]any)
	settings["accounts"] = append(accounts, map[string]any{"user": "window@test", "pass": tuic.SocksPassword()})
	next.Settings = mustJSON(t, settings)
	conn, err := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", apiPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	proxy := &delayedTuicHandler{client: command.NewHandlerServiceClient(conn), entered: make(chan struct{}), resume: make(chan struct{})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	command.RegisterHandlerServiceServer(grpcServer, proxy)
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()
	var resumeOnce sync.Once
	resume := func() { resumeOnce.Do(func() { close(proxy.resume) }) }
	defer resume()
	api := &xray.XrayAPI{}
	if err := api.Init(listener.Addr().(*net.TCPAddr).Port); err != nil {
		t.Fatal(err)
	}
	defer api.Close()
	instances, err := (&InboundService{}).DesiredTuicInstances()
	if err != nil || len(instances) != 1 {
		t.Fatalf("native instance missing: %+v %v", instances, err)
	}
	newClient := model.Client{Email: "window@test", ID: uuid.NewString(), Password: "window-password", Enable: true}
	native.UpdateUsers(append(append([]tuic.TuicClientSettings(nil), instances[0].Clients...), tuic.TuicClientSettings{
		UUID: newClient.ID, Email: newClient.Email, Password: newClient.Password,
	}))
	defer native.UpdateUsers(instances[0].Clients)
	newPeer := connectTuicRoutingClient(t, instances[0], newClient)
	newUserResult := make(chan error, 1)
	go func() { newUserResult <- exchangeTuicRoute(newPeer, "tcp", "fallback:"+tuicRoutingProbe) }()
	select {
	case err := <-newUserResult:
		t.Fatalf("new account did not wait for relay configuration: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	persistent := connectTuicRoutingClient(t, instances[0], clients[1])
	probeTuicRoute(t, persistent, "native", "user:"+tuicRoutingProbe)
	tcpClient := connectTuicRoutingClient(t, instances[0], clients[0])
	streamClient := connectTuicRoutingClient(t, instances[0], clients[2])
	updated := make(chan error, 1)
	go func() {
		updated <- applyTuicRelayReplacements(api, []xray.InboundReplacement{{Previous: &previous, Next: &next}})
	}()
	select {
	case <-proxy.entered:
	case err := <-updated:
		t.Fatalf("replacement failed before the controlled window: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("replacement did not remove the old listener")
	}
	tcpResult, udpResult := make(chan error, 1), make(chan error, 1)
	go func() { tcpResult <- exchangeTuicRoute(tcpClient, "tcp", "specific:"+tuicRoutingProbe) }()
	go func() { udpResult <- exchangeTuicRoute(streamClient, "quic", "fallback:"+tuicRoutingProbe) }()
	var packet bytes.Buffer
	target := &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(192, 0, 2, 1), Port: 6697}
	if err := tuic.WritePacket(&packet, 99, 1, 1, 0, target, []byte(tuicRoutingProbe)); err != nil {
		t.Fatal(err)
	}
	if err := persistent.SendDatagram(packet.Bytes()); err != nil {
		t.Fatal(err)
	}
	// A pending association must not block the same QUIC connection's existing UDP flow.
	probeTuicRoute(t, persistent, "native", "user:"+tuicRoutingProbe)
	resume()
	if err := <-updated; err != nil {
		t.Fatal(err)
	}
	for _, result := range []<-chan error{tcpResult, udpResult, newUserResult} {
		if err := <-result; err != nil {
			t.Errorf("new TUIC request failed during relay replacement: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response, err := persistent.ReceiveDatagram(ctx)
	if err != nil {
		t.Fatalf("first packet of the queued native UDP association was lost: %v", err)
	}
	reader := bytes.NewReader(response)
	_, _, err = tuic.ReadCommand(reader)
	if err != nil {
		t.Fatal(err)
	}
	header, err := tuic.ReadPacketHeader(reader)
	body, bodyErr := io.ReadAll(reader)
	if err != nil || bodyErr != nil || header.AssocID != 99 || string(body) != "user:"+tuicRoutingProbe {
		t.Fatalf("queued UDP response mismatch: header=%+v body=%q errors=%v/%v", header, body, err, bodyErr)
	}
	if err := applyTuicRelayReplacements(api, []xray.InboundReplacement{{Previous: &next, Next: &previous}}); err != nil {
		t.Fatal(err)
	}
	proxy.reject.Store(true)
	err = applyTuicRelayReplacements(api, []xray.InboundReplacement{{Previous: &previous, Next: &next}})
	if err == nil || strings.Count(err.Error(), "controlled relay add failure") != 2 {
		t.Fatalf("relay update and rollback failures were hidden: %v", err)
	}
	recovery := make(chan error, 1)
	go func() { recovery <- exchangeTuicRoute(tcpClient, "tcp", "specific:"+tuicRoutingProbe) }()
	select {
	case err := <-recovery:
		t.Fatalf("failed rollback admitted a request into the missing relay: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := repairUnavailableTuicRelays(cfg, apiPort); err != nil {
		t.Fatal(err)
	}
	if err := <-recovery; err != nil {
		t.Fatalf("unavailable relay did not recover with unchanged desired config: %v", err)
	}
}
