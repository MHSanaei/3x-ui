package job

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	stats "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type trafficRetryStats struct {
	stats.UnimplementedStatsServiceServer
	counter atomic.Int64
}

func (s *trafficRetryStats) QueryStats(_ context.Context, req *stats.QueryStatsRequest) (*stats.QueryStatsResponse, error) {
	keys := map[string]string{"inbound>>>": "retry-in", "outbound>>>": "retry-out", "user>>>": "retry@example"}
	value := s.counter.Load()
	name := req.Pattern + keys[req.Pattern] + ">>>traffic>>>"
	return &stats.QueryStatsResponse{Stat: []*stats.Stat{
		{Name: name + "uplink", Value: value},
		{Name: name + "downlink", Value: 2 * value},
	}}, nil
}

func startTrafficRetryStats(t *testing.T) (*trafficRetryStats, *grpc.Server, int) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	fake := new(trafficRetryStats)
	stats.RegisterStatsServiceServer(server, fake)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	return fake, server, lis.Addr().(*net.TCPAddr).Port
}

func TestXrayTrafficPollRetriesRolledBackSample(t *testing.T) {
	testXrayTrafficPollRetriesRolledBackSample(t)
}

func TestXrayTrafficPollRetriesRolledBackSamplePostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	cleanup, err := testpg.IsolatePackage("xray_traffic_retry")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	testXrayTrafficPollRetriesRolledBackSample(t)
}

func testXrayTrafficPollRetriesRolledBackSample(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	dbtest.InitDB(t, filepath.Join(dir, "x-ui.db"))
	db := database.GetDB()
	ib := model.Inbound{Tag: "retry-in", Protocol: model.VLESS, Enable: true, Settings: "{}"}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatal(err)
	}
	ct := xray.ClientTraffic{Email: "retry@example", InboundId: ib.Id, Enable: true}
	if err := db.Create(&ct).Error; err != nil {
		t.Fatal(err)
	}

	fake, server, port := startTrafficRetryStats(t)
	api := new(xray.XrayAPI)
	if err := api.Init(port); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(api.Close)
	job := NewXrayTrafficJob()
	poll := func(counter int64) bool {
		fake.counter.Store(counter)
		sample, _, _ := job.pollTraffic(api.GetTraffic)
		return sample != nil
	}

	assertTotals := func(t *testing.T, up int64) {
		t.Helper()
		var inbound model.Inbound
		var client xray.ClientTraffic
		var outbound model.OutboundTraffics
		if err := db.First(&inbound, ib.Id).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&client, ct.Id).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Where("tag = ?", "retry-out").Find(&outbound).Error; err != nil {
			t.Fatal(err)
		}
		got := [6]int64{inbound.Up, inbound.Down, client.Up, client.Down, outbound.Up, outbound.Down}
		if want := [6]int64{up, 2 * up, up, 2 * up, up, 2 * up}; got != want {
			t.Errorf("inbound/client/outbound counters = %v, want %v", got, want)
		}
	}

	poll(200)
	assertTotals(t, 0)
	const fail = "test:rollback_xray_sample"
	blocked := false
	callback := db.Callback().Raw()
	if err := callback.Before("gorm:raw").Register(fail, func(tx *gorm.DB) {
		if strings.HasPrefix(tx.Statement.SQL.String(), "UPDATE client_traffics SET up =") {
			blocked = true
			_ = tx.AddError(errors.New("injected client traffic write failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	poll(300)
	if !blocked {
		t.Fatal("failure injection did not reach the traffic write")
	}
	assertTotals(t, 0)
	if err := callback.Remove(fail); err != nil {
		t.Fatal(err)
	}

	// The core can be down while the writer recovers. Retry without a stats connection.
	api.Close()
	if !poll(300) {
		t.Fatal("pending sample did not commit while the collector was unavailable")
	}
	assertTotals(t, 100)
	if err := api.Init(port); err != nil {
		t.Fatal(err)
	}
	poll(300)
	assertTotals(t, 100)
	api.Close()
	server.Stop()
	fake, _, port = startTrafficRetryStats(t)
	if err := api.Init(port); err != nil {
		t.Fatal(err)
	}
	poll(30)
	assertTotals(t, 130)
	poll(40)
	assertTotals(t, 140)

	for i, mode := range []string{"error", "panic"} {
		t.Run("post-commit "+mode, func(t *testing.T) {
			const name = "test:fail_xray_lifecycle"
			reached := false
			query := db.Callback().Query()
			if err := query.After("gorm:query").Register(name, func(tx *gorm.DB) {
				if strings.Contains(tx.Statement.SQL.String(), "reset_max <= 0 or reset_count < reset_max") {
					reached = true
					if mode == "panic" {
						panic("injected traffic maintenance panic")
					}
					_ = tx.AddError(errors.New("injected traffic maintenance error"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			counter := 50 + int64(i)*10
			ok := poll(counter)
			if err := query.Remove(name); err != nil {
				t.Fatal(err)
			}
			if !reached || !ok {
				t.Fatal("accounting should be acknowledged despite a failure in later maintenance")
			}
			assertTotals(t, 100+counter)
			poll(counter)
			assertTotals(t, 100+counter)
		})
	}
}
