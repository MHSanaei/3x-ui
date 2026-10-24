package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	_ "github.com/xtls/xray-core/app/commander"
	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/infra/conf"
	_ "github.com/xtls/xray-core/proxy/blackhole"
	_ "github.com/xtls/xray-core/proxy/dokodemo"
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
	apiPort := freeLoopbackPort(t)
	template := map[string]any{
		"api":      map[string]any{"tag": "api", "services": []string{"HandlerService"}},
		"inbounds": []any{map[string]any{"tag": "api", "listen": "127.0.0.1", "port": apiPort, "protocol": "tunnel", "settings": map[string]any{"address": "127.0.0.1"}}},
		"log":      map[string]any{"loglevel": "error", "access": "none"}, "stats": map[string]any{},
		"outbounds": outbounds,
		"policy": map[string]any{
			"levels": map[string]any{
				"0":          map[string]any{"statsUserUplink": true, "statsUserDownlink": true, "connIdle": 123},
				"4294967295": map[string]any{"statsUserUplink": true, "statsUserDownlink": true},
			},
			"system": map[string]any{"statsInboundUplink": true, "statsInboundDownlink": true},
		},
		"routing": map[string]any{"rules": []any{
			map[string]any{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
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
	var relayInbound xray.InboundConfig
	for _, candidate := range cfg.InboundConfigs {
		if candidate.Tag == ib.Tag {
			relayInbound = candidate
		}
	}
	if err := json.Unmarshal(relayInbound.Settings, &relaySettings); err != nil {
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
	if err := publishTuicRelayReadiness(cfg); err != nil {
		t.Fatal(err)
	}
	counters := x.GetFeature(stats.ManagerType()).(stats.Manager)
	instances, err := (&InboundService{}).DesiredTuicInstances()
	if err != nil || len(instances) != 1 {
		t.Fatalf("desired TUIC instances = %v, error %v", instances, err)
	}
	server, err := tuic.NewServer(instances[0], &tuic.SocksRelay{
		Gate: tuic.ManagedRelayGate(tuic.SOCKSPortForInbound(ib.Id)), InboundID: ib.Id,
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
	for _, reset := range []string{"telegram", "scheduled"} {
		t.Run("restore-"+reset, func(t *testing.T) {
			if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", clients[0].Email).Update("enable", false).Error; err != nil {
				t.Fatal(err)
			}
			server.UpdateUsers(nil)
			var resetErr error
			if reset == "telegram" {
				resetErr = (&InboundService{}).ResetClientTrafficByEmail(clients[0].Email)
			} else {
				_, resetErr = (&ClientService{}).ResetAllClientTraffics(&InboundService{}, ib.Id)
			}
			if resetErr != nil {
				t.Fatal(resetErr)
			}
			renewed, err := (&InboundService{}).DesiredTuicInstances()
			if err != nil || len(renewed) != 1 {
				t.Fatalf("renewal missing: %+v %v", renewed, err)
			}
			server.UpdateUsers(renewed[0].Clients)
			peer := connectTuicRoutingClient(t, renewed[0], clients[0])
			for _, mode := range []string{"tcp", "native", "quic"} {
				probeTuicRoute(t, peer, mode, "specific:"+tuicRoutingProbe)
			}
		})
	}

	t.Run("replacement-window", func(t *testing.T) { verifyTuicReplacementWindow(t, cfg, apiPort, ib, server, clients) })

	verifyTuicHotUserUpdates(t, cfg, apiPort, ib, server, clients, counters)

	priorOrdinary := int64(0)
	if counter := counters.GetCounter("user>>>" + clients[1].Email + ">>>traffic>>>uplink"); counter != nil {
		priorOrdinary = counter.Value()
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
	if counter == nil || counter.Value()-priorOrdinary != int64(len(tuicRoutingProbe)) {
		t.Fatal("ordinary Xray user traffic must still be counted independently of TUIC")
	}
}

// listenTCPAndUDPOnOnePort retries because a free TCP port may be taken or reserved for UDP
// (Windows excludes port ranges per protocol).
func listenTCPAndUDPOnOnePort(t *testing.T) (net.Listener, net.PacketConn) {
	t.Helper()
	for range 20 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		if packet, err := net.ListenPacket("udp", listener.Addr().String()); err == nil {
			return listener, packet
		}
		_ = listener.Close()
	}
	t.Fatal("no port free for both TCP and UDP")
	return nil, nil
}

func startTuicRouteMarker(t *testing.T, marker string) string {
	t.Helper()
	listener, packet := listenTCPAndUDPOnOnePort(t)
	t.Cleanup(func() { _ = listener.Close() })
	t.Cleanup(func() { _ = packet.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
				body := make([]byte, len(tuicRoutingProbe))
				for {
					if _, err := io.ReadFull(conn, body); err != nil {
						return
					}
					if _, err := conn.Write(append([]byte(marker+":"), body...)); err != nil {
						return
					}
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

func verifyTuicHotUserUpdates(t *testing.T, initial *xray.Config, apiPort int, inbound *model.Inbound, server *tuic.Server, clients []model.Client, counters stats.Manager) {
	t.Helper()
	svc := &XrayService{}
	if err := (&SettingService{}).SetRestartXrayOnClientDisable(true); err != nil {
		t.Fatal(err)
	}
	api := &xray.XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		t.Fatal(err)
	}
	defer api.Close()
	ordinary := initial.InboundConfigs[len(initial.InboundConfigs)-1]
	otherRelay := &tuic.SocksRelay{Addr: fmt.Sprintf("127.0.0.1:%d", ordinary.Port), Password: "ordinary-password"}
	other, err := otherRelay.DialTCP(context.Background(), clients[1].Email, &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(192, 0, 2, 1), Port: 6697})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	assertOtherSurvives := func() {
		t.Helper()
		_ = other.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := other.Write([]byte(tuicRoutingProbe)); err != nil {
			t.Fatal(err)
		}
		response := make([]byte, len("user:"+tuicRoutingProbe))
		if _, err := io.ReadFull(other, response); err != nil || string(response) != "user:"+tuicRoutingProbe {
			t.Fatalf("TUIC account edit interrupted an unrelated live connection: %q, %v", response, err)
		}
	}
	assertOtherSurvives()
	instances, err := (&InboundService{}).DesiredTuicInstances()
	if err != nil || len(instances) != 1 {
		t.Fatalf("missing native instance: %+v, %v", instances, err)
	}
	persistent := connectTuicRoutingClient(t, instances[0], clients[1])
	assertNativeSurvives := func() {
		t.Helper()
		for _, mode := range []string{"native", "quic"} {
			probeTuicRoute(t, persistent, mode, "user:"+tuicRoutingProbe)
		}
	}
	assertNativeSurvives()
	t.Run("hot-rollback", func(t *testing.T) {
		var previous xray.InboundConfig
		for _, candidate := range initial.InboundConfigs {
			if candidate.TuicRelay {
				previous = candidate
			}
		}
		invalid := previous
		var settings map[string]any
		if err := json.Unmarshal(previous.Settings, &settings); err != nil {
			t.Fatal(err)
		}
		settings["accounts"] = []relayAccount{{User: strings.Repeat("x", 256), Pass: "secret"}}
		invalid.Settings = mustJSON(t, settings)
		err := applyTuicRelayReplacements(api, []xray.InboundReplacement{{Previous: &previous, Next: &invalid}})
		if err == nil || !strings.Contains(err.Error(), "credentials exceed SOCKS5 bounds") {
			t.Fatalf("invalid replacement was not refused: %v", err)
		}
		assertOtherSurvives()
		assertNativeSurvives()
		fresh := connectTuicRoutingClient(t, instances[0], clients[0])
		for _, mode := range []string{"tcp", "native", "quic"} {
			probeTuicRoute(t, fresh, mode, "specific:"+tuicRoutingProbe)
		}
	})
	added := model.Client{Email: "добавлен@test", ID: uuid.NewString(), Password: "new-password", Enable: true}
	before := initial
	for _, step := range []string{"add", "delete", "rename", "empty"} {
		t.Run("hot-"+step, func(t *testing.T) {
			switch step {
			case "add":
				clients = append(clients, added)
			case "delete":
				clients = clients[1:]
			case "rename":
				added.Email = "переименован@test"
				clients[len(clients)-1] = added
			case "empty":
				clients = nil
			}
			var settings map[string]any
			if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
				t.Fatal(err)
			}
			settings["clients"] = clients
			if err := database.GetDB().Model(inbound).Update("settings", string(mustJSON(t, settings))).Error; err != nil {
				t.Fatal(err)
			}
			after, err := svc.GetXrayConfig()
			if err != nil {
				t.Fatal(err)
			}
			after.InboundConfigs = append(after.InboundConfigs, ordinary)
			diff, hot := xray.ComputeHotDiff(before, after)
			if !hot || svc.restartToDropClients(diff) || applyTuicRelayReplacements(api, diff.TuicRelayChanges) != nil || !applyXrayHotDiff(api, diff) {
				t.Fatalf("account update requires a core restart: hot=%t diff=%+v", hot, diff)
			}
			native, err := (&InboundService{}).DesiredTuicInstances()
			if err != nil {
				t.Fatal(err)
			}
			if len(native) > 0 {
				server.UpdateUsers(native[0].Clients)
				peer := connectTuicRoutingClient(t, native[0], added)
				for _, mode := range []string{"tcp", "native", "quic"} {
					probeTuicRoute(t, peer, mode, "fallback:"+tuicRoutingProbe)
				}
				if counter := counters.GetCounter("user>>>" + added.Email + ">>>traffic>>>uplink"); counter != nil {
					t.Fatalf("hot-added user lost relay policy and was billed twice: %d", counter.Value())
				}
			} else {
				server.UpdateUsers(nil)
			}
			if step != "empty" {
				assertNativeSurvives()
			}
			assertOtherSurvives()
			before = after
		})
	}
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
	if err := exchangeTuicRoute(client, mode, want); err != nil {
		t.Fatalf("%s relay request failed: %v", mode, err)
	}
}

func exchangeTuicRoute(client *quic.Conn, mode, want string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	target := &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(192, 0, 2, 1), Port: 6697}
	var frame bytes.Buffer
	var reader io.Reader
	switch mode {
	case "tcp":
		stream, err := client.OpenStreamSync(ctx)
		if err != nil {
			return err
		}
		defer stream.Close()
		_ = stream.SetDeadline(time.Now().Add(5 * time.Second))
		frame.Write([]byte{tuic.ProtocolVersion, tuic.CmdConnect})
		if err := tuic.WriteAddress(&frame, target); err != nil {
			return err
		}
		frame.WriteString(tuicRoutingProbe)
		if _, err := stream.Write(frame.Bytes()); err != nil {
			return err
		}
		reader = stream
	case "native", "quic":
		association := uint16(1)
		if mode == "quic" {
			association = 2
		}
		if err := tuic.WritePacket(&frame, association, 1, 1, 0, target, []byte(tuicRoutingProbe)); err != nil {
			return err
		}
		if mode == "native" {
			if err := client.SendDatagram(frame.Bytes()); err != nil {
				return err
			}
			response, err := client.ReceiveDatagram(ctx)
			if err != nil {
				return err
			}
			reader = bytes.NewReader(response)
		} else {
			stream, err := client.OpenUniStreamSync(ctx)
			if err != nil {
				return err
			}
			if _, err := stream.Write(frame.Bytes()); err != nil {
				return err
			}
			_ = stream.Close()
			response, err := client.AcceptUniStream(ctx)
			if err != nil {
				return err
			}
			_ = response.SetReadDeadline(time.Now().Add(5 * time.Second))
			reader = response
		}
		_, cmd, err := tuic.ReadCommand(reader)
		if err != nil {
			return fmt.Errorf("read UDP response command: %w", err)
		}
		if cmd != tuic.CmdPacket {
			return fmt.Errorf("UDP response command = %d, want Packet", cmd)
		}
		hdr, err := tuic.ReadPacketHeader(reader)
		if err != nil {
			return fmt.Errorf("read UDP response header: %w", err)
		}
		if hdr.AssocID != association || hdr.Size != uint16(len(want)) {
			return fmt.Errorf("UDP response header = %+v, want association %d and size %d", hdr, association, len(want))
		}
	}
	response := make([]byte, len(want))
	if _, err := io.ReadFull(reader, response); err != nil {
		return err
	}
	if string(response) != want {
		return fmt.Errorf("%s route returned %q, want %q", mode, response, want)
	}
	return nil
}
