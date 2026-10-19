package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/infra/conf"
	_ "github.com/xtls/xray-core/proxy/blackhole"
	_ "github.com/xtls/xray-core/proxy/freedom"
	_ "github.com/xtls/xray-core/proxy/socks"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const tuicRoutingProbe = "tuic-route-probe"

func TestTuicUserRoutingDoesNotDoubleCountTraffic(t *testing.T) {
	setupBulkDB(t)
	cert, key := writeTestCertificate(t)
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := udp.LocalAddr().(*net.UDPAddr).Port
	_ = udp.Close()
	clients := []model.Client{
		{Email: "клиент@test", ID: uuid.NewString(), Password: "specific-password", Enable: true},
		{Email: "user@test", ID: uuid.NewString(), Password: "user-password", Enable: true},
		{Email: "fallback@test", ID: uuid.NewString(), Password: "fallback-password", Enable: true},
	}
	settings := map[string]any{"certificate": cert, "private_key": key, "clients": clients, "alpn": []string{"h3"}}
	ib := mkInbound(t, port, model.TUIC, string(mustJSON(t, settings)))
	ib.Listen = "127.0.0.1"
	if err := database.GetDB().Save(ib).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatal(err)
	}
	for _, c := range clients {
		if err := (&InboundService{}).AddClientStat(database.GetDB(), ib.Id, &c); err != nil {
			t.Fatal(err)
		}
	}

	outbounds := []any{map[string]any{"tag": "blocked", "protocol": "blackhole"}}
	for _, tag := range []string{"specific", "user", "fallback"} {
		addr := startTuicRouteMarker(t, tag)
		outbounds = append(outbounds, map[string]any{"tag": tag, "protocol": "freedom", "settings": map[string]any{"redirect": addr}})
	}
	template := map[string]any{
		"log": map[string]any{"loglevel": "error", "access": "none"}, "stats": map[string]any{},
		"outbounds": outbounds,
		"policy": map[string]any{
			"levels": map[string]any{
				"0":          map[string]any{"statsUserUplink": true, "statsUserDownlink": true, "connIdle": 123},
				"4294967295": map[string]any{"statsUserUplink": true, "statsUserDownlink": true},
			},
			"system": map[string]any{"statsInboundUplink": true, "statsInboundDownlink": true},
		},
		"routing": map[string]any{"rules": []any{
			map[string]any{"type": "field", "inboundTag": []string{ib.Tag}, "user": []string{clients[0].Email}, "outboundTag": "specific"},
			map[string]any{"type": "field", "user": []string{clients[0].Email, clients[1].Email}, "outboundTag": "user"},
			map[string]any{"type": "field", "inboundTag": []string{ib.Tag}, "outboundTag": "fallback"},
		}},
	}
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", string(mustJSON(t, template))); err != nil {
		t.Fatal(err)
	}
	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	ordinaryPort := freeLoopbackPort(t)
	cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
		Listen: mustJSON(t, "127.0.0.1"), Port: ordinaryPort, Protocol: "socks", Tag: "ordinary",
		Settings: mustJSON(t, map[string]any{"auth": "password", "accounts": []any{map[string]any{"user": clients[1].Email, "pass": "ordinary-password"}}}),
	})
	var parsed conf.Config
	if err := json.Unmarshal(mustJSON(t, cfg), &parsed); err != nil {
		t.Fatal(err)
	}
	var relaySettings struct {
		UserLevel uint32 `json:"userLevel"`
	}
	if err := json.Unmarshal(cfg.InboundConfigs[0].Settings, &relaySettings); err != nil {
		t.Fatal(err)
	}
	relayPolicy := parsed.Policy.Levels[relaySettings.UserLevel]
	if relaySettings.UserLevel == 0 || relaySettings.UserLevel == ^uint32(0) || relayPolicy == nil ||
		relayPolicy.ConnectionIdle == nil || *relayPolicy.ConnectionIdle != 123 ||
		relayPolicy.StatsUserUplink || relayPolicy.StatsUserDownlink || relayPolicy.StatsUserOnline {
		t.Fatalf("relay policy lost session limits, collided with a custom level, or enabled duplicate stats: level=%d policy=%+v", relaySettings.UserLevel, relayPolicy)
	}
	if !parsed.Policy.Levels[0].StatsUserUplink || !parsed.Policy.Levels[^uint32(0)].StatsUserUplink {
		t.Fatal("TUIC relay altered existing Xray user policy levels")
	}
	built, err := parsed.Build()
	if err != nil {
		t.Fatal(err)
	}
	x, err := core.New(built)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = x.Close() })
	if err := x.Start(); err != nil {
		t.Fatal(err)
	}
	counters := x.GetFeature(stats.ManagerType()).(stats.Manager)
	instances, err := (&InboundService{}).DesiredTuicInstances()
	if err != nil || len(instances) != 1 {
		t.Fatalf("desired TUIC instances = %v, error %v", instances, err)
	}
	server, err := tuic.NewServer(instances[0], &tuic.SocksRelay{
		Addr: fmt.Sprintf("127.0.0.1:%d", tuic.SOCKSPortForInbound(ib.Id)), Password: tuic.SocksPassword(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	for _, credentials := range []struct{ email, password string }{
		{"unknown@test", tuic.SocksPassword()}, {clients[0].Email, "wrong-password"},
	} {
		relay := &tuic.SocksRelay{Addr: fmt.Sprintf("127.0.0.1:%d", tuic.SOCKSPortForInbound(ib.Id)), Password: credentials.password}
		conn, err := relay.DialTCP(context.Background(), credentials.email, &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(192, 0, 2, 1), Port: 6697})
		if conn != nil {
			_ = conn.Close()
		}
		if err == nil || err.Error() != "tuic socks: auth rejected (code 255)" {
			t.Fatalf("invalid relay credentials were not rejected: %v", err)
		}
	}
	for i, route := range []string{"specific", "user", "fallback"} {
		t.Run(route, func(t *testing.T) {
			client := connectTuicRoutingClient(t, instances[0], clients[i])
			for _, mode := range []string{"tcp", "native", "quic"} {
				t.Run(mode, func(t *testing.T) { probeTuicRoute(t, client, mode, route+":"+tuicRoutingProbe) })
			}
			up, down := int64(3*len(tuicRoutingProbe)), int64(3*len(route+":"+tuicRoutingProbe))
			var gotUp, gotDown int64
			deadline := time.Now().Add(3 * time.Second)
			for gotUp < up || gotDown < down {
				for _, delta := range server.CollectClientTraffic() {
					if delta.Email != clients[i].Email {
						t.Fatalf("unexpected native traffic identity: %+v", delta)
					}
					gotUp += delta.Up
					gotDown += delta.Down
				}
				if time.Now().After(deadline) {
					t.Fatalf("native traffic = %d/%d, want %d/%d", gotUp, gotDown, up, down)
				}
				time.Sleep(time.Millisecond)
			}
			if gotUp != up || gotDown != down {
				t.Fatalf("native traffic = %d/%d, want %d/%d", gotUp, gotDown, up, down)
			}
			if _, _, err := (&InboundService{}).AddTraffic(nil, []*xray.ClientTraffic{{Email: clients[i].Email, Up: gotUp, Down: gotDown}}); err != nil {
				t.Fatal(err)
			}
			var billed xray.ClientTraffic
			if err := database.GetDB().Where("email = ?", clients[i].Email).First(&billed).Error; err != nil {
				t.Fatal(err)
			}
			if billed.Up != up || billed.Down != down {
				t.Fatalf("persisted TUIC usage = %d/%d, want %d/%d", billed.Up, billed.Down, up, down)
			}
			for _, direction := range []string{"uplink", "downlink"} {
				if counter := counters.GetCounter("user>>>" + clients[i].Email + ">>>traffic>>>" + direction); counter != nil {
					t.Fatalf("TUIC relay created duplicate Xray %s counter: %d", direction, counter.Value())
				}
			}
			if online := counters.GetOnlineMap("user>>>" + clients[i].Email + ">>>online"); online != nil {
				t.Fatal("TUIC relay exposed loopback addresses as user online IPs")
			}
			if counter := counters.GetCounter("inbound>>>" + ib.Tag + ">>>traffic>>>uplink"); counter == nil || counter.Value() < up {
				t.Fatal("TUIC inbound totals were lost while disabling duplicate per-user counters")
			}
		})
	}

	relay := &tuic.SocksRelay{Addr: fmt.Sprintf("127.0.0.1:%d", ordinaryPort), Password: "ordinary-password"}
	conn, err := relay.DialTCP(context.Background(), clients[1].Email, &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(192, 0, 2, 1), Port: 6697})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(tuicRoutingProbe)); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len("user:"+tuicRoutingProbe))
	if _, err := io.ReadFull(conn, response); err != nil || string(response) != "user:"+tuicRoutingProbe {
		t.Fatalf("ordinary SOCKS route = %q, error %v", response, err)
	}
	counter := counters.GetCounter("user>>>" + clients[1].Email + ">>>traffic>>>uplink")
	if counter == nil || counter.Value() != int64(len(tuicRoutingProbe)) {
		t.Fatal("ordinary Xray user traffic must still be counted independently of TUIC")
	}
}

func startTuicRouteMarker(t *testing.T, marker string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	packet, err := net.ListenPacket("udp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = packet.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				body := make([]byte, len(tuicRoutingProbe))
				if _, err := io.ReadFull(conn, body); err == nil {
					_, _ = conn.Write(append([]byte(marker+":"), body...))
				}
			}()
		}
	}()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := packet.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = packet.WriteTo(append([]byte(marker+":"), buf[:n]...), addr)
		}
	}()
	return listener.Addr().String()
}

func connectTuicRoutingClient(t *testing.T, inst tuic.Instance, user model.Client) *quic.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := quic.DialAddr(ctx, inst.BindTo(), &tls.Config{InsecureSkipVerify: true, NextProtos: inst.ALPN}, &quic.Config{EnableDatagrams: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.CloseWithError(0, "complete") })
	id := uuid.MustParse(user.ID)
	tlsState := client.ConnectionState().TLS
	token, err := tlsState.ExportKeyingMaterial(string(id[:]), []byte(user.Password), 32)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.OpenUniStreamSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Write(append(append([]byte{tuic.ProtocolVersion, tuic.CmdAuthenticate}, id[:]...), token...)); err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
	return client
}

func probeTuicRoute(t *testing.T, client *quic.Conn, mode, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	target := &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(192, 0, 2, 1), Port: 6697}
	var frame bytes.Buffer
	var reader io.Reader
	switch mode {
	case "tcp":
		stream, err := client.OpenStreamSync(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer stream.Close()
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
		frame.Write([]byte{tuic.ProtocolVersion, tuic.CmdConnect})
		if err := tuic.WriteAddress(&frame, target); err != nil {
			t.Fatal(err)
		}
		frame.WriteString(tuicRoutingProbe)
		if _, err := stream.Write(frame.Bytes()); err != nil {
			t.Fatal(err)
		}
		reader = stream
	case "native", "quic":
		association := uint16(1)
		if mode == "quic" {
			association = 2
		}
		if err := tuic.WritePacket(&frame, association, 1, 1, 0, target, []byte(tuicRoutingProbe)); err != nil {
			t.Fatal(err)
		}
		if mode == "native" {
			if err := client.SendDatagram(frame.Bytes()); err != nil {
				t.Fatal(err)
			}
			response, err := client.ReceiveDatagram(ctx)
			if err != nil {
				t.Fatal(err)
			}
			reader = bytes.NewReader(response)
		} else {
			stream, err := client.OpenUniStreamSync(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stream.Write(frame.Bytes()); err != nil {
				t.Fatal(err)
			}
			_ = stream.Close()
			response, err := client.AcceptUniStream(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.SetReadDeadline(time.Now().Add(5 * time.Second))
			reader = response
		}
		if _, cmd, err := tuic.ReadCommand(reader); err != nil || cmd != tuic.CmdPacket {
			t.Fatalf("UDP response command = %d, error %v", cmd, err)
		}
		hdr, err := tuic.ReadPacketHeader(reader)
		if err != nil || hdr.AssocID != association || hdr.Size != uint16(len(want)) {
			t.Fatalf("UDP response header = %+v, error %v", hdr, err)
		}
	}
	response := make([]byte, len(want))
	if _, err := io.ReadFull(reader, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != want {
		t.Fatalf("%s route returned %q, want %q", mode, response, want)
	}
}
