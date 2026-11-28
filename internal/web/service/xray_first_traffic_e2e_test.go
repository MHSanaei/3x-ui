package service

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	stats "github.com/xtls/xray-core/app/stats/command"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestManagedXrayFirstTrafficAfterStartAndReplacement(t *testing.T) {
	bin := os.Getenv("XRAY_E2E_BINARY")
	if bin == "" {
		t.Skip("set XRAY_E2E_BINARY to run the real-core traffic lifecycle")
	}
	db := initTrafficTestDB(t)
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	t.Setenv("XUI_LOG_FOLDER", dir)
	t.Cleanup(func() {
		if t.Failed() {
			data, _ := os.ReadFile(filepath.Join(dir, "error.log"))
			t.Log(string(data))
		}
	})
	if err := os.Symlink(bin, filepath.Join(dir, xray.GetBinaryName())); err != nil {
		t.Fatal(err)
	}
	previous, _ := xrayState.snapshot()
	t.Cleanup(func() { xrayState.replace(previous) })

	const email = "managed-first-traffic@example.test"
	clientID := uuid.New()
	apiPort, proxyPort := freeLoopbackPort(t), freeLoopbackPort(t)
	ib := &model.Inbound{Tag: "traffic-probe", Port: proxyPort, Protocol: model.VLESS, Enable: true, Settings: "{}"}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	client := &xray.ClientTraffic{InboundId: ib.Id, Email: email, Up: 150, Down: 300, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatal(err)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "traffic fixture")
	}))
	defer origin.Close()
	originPort := origin.Listener.Addr().(*net.TCPAddr).Port
	cfg := &xray.Config{
		LogConfig:       mustJSON(t, map[string]string{"loglevel": "debug", "error": filepath.Join(dir, "error.log"), "access": filepath.Join(dir, "access.log")}),
		RouterConfig:    []byte(`{"rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"}]}`),
		OutboundConfigs: []byte(`[{"protocol":"freedom","tag":"direct","settings":{"finalRules":[{"action":"allow","ip":["127.0.0.1/32"]}]}}]`),
		API:             []byte(`{"tag":"api","services":["StatsService"]}`),
		Stats:           []byte(`{}`),
		Policy:          []byte(`{"levels":{"0":{"statsUserUplink":true,"statsUserDownlink":true}}}`),
		InboundConfigs: []xray.InboundConfig{
			{Listen: mustJSON(t, "127.0.0.1"), Port: apiPort, Protocol: "tunnel", Tag: "api", Settings: []byte(`{"rewriteAddress":"127.0.0.1"}`)},
			{
				Listen: mustJSON(t, "127.0.0.1"), Port: proxyPort, Protocol: "vless", Tag: ib.Tag,
				Settings: fmt.Appendf(nil, `{"decryption":"none","clients":[{"id":%q,"email":%q}]}`, clientID.String(), email),
			},
		},
	}
	poller := &XrayService{}
	wantUp, wantDown := client.Up, client.Down
	for _, requests := range []int{1, 2} {
		process := xray.NewTestProcess(cfg, filepath.Join(dir, strconv.Itoa(requests)+".json"))
		if err := process.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = process.Stop() })
		xrayState.replace(process)
		probe := &xray.XrayAPI{}
		if err := probe.Init(apiPort); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(probe.Close)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		t.Cleanup(cancel)
		var up, down int64
		query := func() error {
			out, err := (*probe.StatsServiceClient).QueryStats(ctx, &stats.QueryStatsRequest{Pattern: "user>>>"})
			if err == nil {
				up, down = 0, 0
				for _, counter := range out.GetStat() {
					switch counter.GetName() {
					case "user>>>" + email + ">>>traffic>>>uplink":
						up = counter.GetValue()
					case "user>>>" + email + ">>>traffic>>>downlink":
						down = counter.GetValue()
					}
				}
			}
			return err
		}
		for err := query(); err != nil; err = query() {
			if ctx.Err() != nil {
				t.Fatalf("stats API did not start: %v", err)
			}
			time.Sleep(20 * time.Millisecond)
		}
		for range requests {
			sendManagedTrafficProbe(t, clientID, proxyPort, originPort)
		}
		if err := query(); err != nil || up <= 0 || down <= 0 {
			t.Fatalf("non-resetting probe up=%d down=%d err=%v", up, down, err)
		}
		wantUp, wantDown = wantUp+up, wantDown+down

		for range 2 {
			inbounds, stats, err := poller.GetXrayTraffic()
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = (&InboundService{}).AddXrayTrafficBatch(uuid.NewString(), inbounds, stats)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.First(client, client.Id).Error; err != nil {
				t.Fatal(err)
			}
			if client.Up != wantUp || client.Down != wantDown {
				t.Fatalf("core %d committed up/down=%d/%d, want %d/%d", requests, client.Up, client.Down, wantUp, wantDown)
			}
		}
		if err := process.Stop(); err != nil {
			t.Fatal(err)
		}
	}
}

func sendManagedTrafficProbe(t *testing.T, clientID uuid.UUID, proxyPort, originPort int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(proxyPort)), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	header := append([]byte{0}, clientID[:]...)
	header = append(header, 0, 1) // no addons, TCP
	header = binary.BigEndian.AppendUint16(header, uint16(originPort))
	header = append(header, 1, 127, 0, 0, 1) // IPv4 loopback
	if _, err := conn.Write(append(header, "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"...)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, make([]byte, 2)); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "traffic fixture" {
		t.Fatalf("origin response = %q, err=%v", body, err)
	}
}
