package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type noopBenchRuntime struct{}

func (noopBenchRuntime) Name() string                                     { return "bench-local" }
func (noopBenchRuntime) AddInbound(context.Context, *model.Inbound) error { return nil }
func (noopBenchRuntime) DelInbound(context.Context, *model.Inbound) error { return nil }
func (noopBenchRuntime) UpdateInbound(context.Context, *model.Inbound, *model.Inbound) error {
	return nil
}

func (noopBenchRuntime) AddUser(context.Context, *model.Inbound, map[string]any) error { return nil }

func (noopBenchRuntime) RemoveUser(context.Context, *model.Inbound, string) error { return nil }

func (noopBenchRuntime) UpdateUser(context.Context, *model.Inbound, string, model.Client) error {
	return nil
}

func (noopBenchRuntime) DeleteUser(context.Context, *model.Inbound, string) error { return nil }

func (noopBenchRuntime) AddClient(context.Context, *model.Inbound, model.Client) error { return nil }

func (noopBenchRuntime) DeleteClient(context.Context, string) error { return nil }

func (noopBenchRuntime) RestartXray(context.Context) error { return nil }

func (noopBenchRuntime) ResetClientTraffic(context.Context, *model.Inbound, string) error {
	return nil
}
func (noopBenchRuntime) ResetInboundTraffic(context.Context, *model.Inbound) error { return nil }
func (noopBenchRuntime) ResetAllTraffics(context.Context) error                    { return nil }

func seedWriteScaleDB(b *testing.B, numInbounds, clientsPerInbound int) []int {
	b.Helper()
	dbtest.InitDB(b, filepath.Join(b.TempDir(), "write_bench.db"))
	prevMgr := runtime.GetManager()
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}})
	mgr.SetLocalRuntimeOverride(noopBenchRuntime{})
	runtime.SetManager(mgr)
	b.Cleanup(func() { runtime.SetManager(prevMgr) })

	db := database.GetDB()
	clients := make([]model.Client, clientsPerInbound)
	records := make([]model.ClientRecord, clientsPerInbound)
	traffics := make([]xray.ClientTraffic, clientsPerInbound)
	for i := range clientsPerInbound {
		email := fmt.Sprintf("seed-%04d@example.com", i)
		uid := fmt.Sprintf("10000000-0000-4000-8000-%012d", i+1)
		subID := fmt.Sprintf("seed-sub-%04d", i)
		clients[i] = model.Client{
			Email:      email,
			ID:         uid,
			Password:   fmt.Sprintf("pw-%012d", i+1),
			SubID:      subID,
			Enable:     true,
			TotalGB:    10 << 30,
			ExpiryTime: 1900000000000,
		}
		records[i] = model.ClientRecord{
			Email:      email,
			UUID:       uid,
			Password:   clients[i].Password,
			SubID:      subID,
			Enable:     true,
			TotalGB:    clients[i].TotalGB,
			ExpiryTime: clients[i].ExpiryTime,
		}
		traffics[i] = xray.ClientTraffic{
			Email:      email,
			Enable:     true,
			Total:      clients[i].TotalGB,
			ExpiryTime: clients[i].ExpiryTime,
		}
	}
	if err := db.CreateInBatches(records, 250).Error; err != nil {
		b.Fatalf("seed records: %v", err)
	}

	inboundIDs := make([]int, 0, numInbounds)
	links := make([]model.ClientInbound, 0, clientsPerInbound*numInbounds)
	for ibIdx := range numInbounds {
		proto := model.Shadowsocks
		raw := map[string]any{"method": "chacha20-ietf-poly1305", "clients": clients}
		if ibIdx == 0 {
			proto = model.VLESS
			raw = map[string]any{"decryption": "none", "clients": clients}
		}
		enc, err := json.Marshal(raw)
		if err != nil {
			b.Fatalf("marshal settings: %v", err)
		}
		ib := model.Inbound{
			UserId:         1,
			Remark:         fmt.Sprintf("wib-%02d", ibIdx),
			Enable:         true,
			Port:           25000 + ibIdx,
			Protocol:       proto,
			Settings:       string(enc),
			StreamSettings: `{"network":"tcp","security":"none"}`,
			Tag:            fmt.Sprintf("winbound-%d", 25000+ibIdx),
		}
		if err := db.Create(&ib).Error; err != nil {
			b.Fatalf("seed inbound %d: %v", ibIdx, err)
		}
		inboundIDs = append(inboundIDs, ib.Id)
		for i := range records {
			links = append(links, model.ClientInbound{ClientId: records[i].Id, InboundId: ib.Id})
		}
	}
	for i := range traffics {
		traffics[i].InboundId = inboundIDs[0]
	}
	if err := db.CreateInBatches(traffics, 250).Error; err != nil {
		b.Fatalf("seed traffics: %v", err)
	}
	if err := db.CreateInBatches(links, 500).Error; err != nil {
		b.Fatalf("seed links: %v", err)
	}
	return inboundIDs
}

func BenchmarkWriteHotPaths(b *testing.B) {
	// 22 inbounds x 1,000 existing clients (~22,000 client_inbounds rows).
	inboundIDs := seedWriteScaleDB(b, 22, 1000)
	clientSvc := &ClientService{}
	inboundSvc := &InboundService{}

	b.Run("CreateClient_22Inbounds", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			i++
			email := fmt.Sprintf("bench-create-%d@example.com", i)
			if _, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{
				Client: model.Client{
					Email:      email,
					SubID:      fmt.Sprintf("bench-sub-%d", i),
					Enable:     true,
					TotalGB:    20 << 30,
					ExpiryTime: 1900000000000,
				},
				InboundIds: inboundIDs,
			}); err != nil {
				b.Fatalf("Create %d: %v", i, err)
			}
		}
	})

	b.Run("BulkCreate_10Clients_22Inbounds", func(b *testing.B) {
		b.ReportAllocs()
		batch := 0
		for b.Loop() {
			batch++
			payloads := make([]ClientCreatePayload, 10)
			for j := range payloads {
				payloads[j] = ClientCreatePayload{
					Client: model.Client{
						Email:      fmt.Sprintf("bench-bulk-%d-%d@example.com", batch, j),
						SubID:      fmt.Sprintf("bench-bsub-%d-%d", batch, j),
						Enable:     true,
						TotalGB:    20 << 30,
						ExpiryTime: 1900000000000,
					},
					InboundIds: inboundIDs,
				}
			}
			res, _, err := clientSvc.BulkCreate(inboundSvc, payloads)
			if err != nil || res.Created != 10 {
				b.Fatalf("BulkCreate batch %d = (%+v, %v)", batch, res, err)
			}
		}
	})

	b.Run("BulkSetEnable_10Clients_22Inbounds", func(b *testing.B) {
		emails := make([]string, 10)
		for j := range emails {
			emails[j] = fmt.Sprintf("seed-%04d@example.com", j)
		}
		enable := false
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			res, _, err := clientSvc.BulkSetEnable(inboundSvc, emails, enable)
			if err != nil || res.Changed != 10 {
				b.Fatalf("BulkSetEnable(%v) = (%+v, %v)", enable, res, err)
			}
			enable = !enable
		}
	})

	b.Run("UpdateMetadataOnly_22Inbounds", func(b *testing.B) {
		rec, err := clientSvc.GetRecordByEmail(nil, "seed-0050@example.com")
		if err != nil {
			b.Fatalf("GetRecordByEmail: %v", err)
		}
		c := *rec.ToClient()
		b.ReportAllocs()
		b.ResetTimer()
		i := 0
		for b.Loop() {
			i++
			c.TotalGB = int64(20+i) << 30
			if _, err := clientSvc.Update(inboundSvc, rec.Id, c, rec.LimitHwid); err != nil {
				b.Fatalf("Update %d: %v", i, err)
			}
		}
	})
}
