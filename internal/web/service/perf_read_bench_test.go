package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedSyntheticScaleDB(b *testing.B, numInbounds, numClients, inboundsPerClient int) (inboundIDs []int, sampleEmail string) {
	b.Helper()
	dbtest.InitDB(b, filepath.Join(b.TempDir(), "bench.db"))
	db := database.GetDB()

	clients := make([]model.Client, numClients)
	records := make([]model.ClientRecord, numClients)
	traffics := make([]xray.ClientTraffic, numClients)
	for i := range numClients {
		email := fmt.Sprintf("user-%04d@example.com", i)
		uid := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		subID := fmt.Sprintf("sub-%04d", i)
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
			Up:         int64(i) * 1024,
			Down:       int64(i) * 4096,
			Total:      clients[i].TotalGB,
			ExpiryTime: clients[i].ExpiryTime,
		}
	}
	if err := db.CreateInBatches(records, 250).Error; err != nil {
		b.Fatalf("seed records: %v", err)
	}

	inboundIDs = make([]int, 0, numInbounds)
	links := make([]model.ClientInbound, 0, numClients*inboundsPerClient)
	for ibIdx := range numInbounds {
		proto := model.Shadowsocks
		rawSettings := map[string]any{
			"method":  "chacha20-ietf-poly1305",
			"clients": clients[:min(numClients, 1000)],
		}
		if ibIdx == 0 {
			proto = model.VLESS
			rawSettings = map[string]any{
				"decryption": "none",
				"clients":    clients[:min(numClients, 1000)],
			}
		}
		encoded, err := json.Marshal(rawSettings)
		if err != nil {
			b.Fatalf("marshal settings: %v", err)
		}
		ib := model.Inbound{
			UserId:         1,
			Remark:         fmt.Sprintf("ib-%02d", ibIdx),
			Enable:         true,
			Port:           20000 + ibIdx,
			Protocol:       proto,
			Settings:       string(encoded),
			StreamSettings: `{"network":"tcp","security":"none"}`,
			Tag:            fmt.Sprintf("inbound-%d", 20000+ibIdx),
		}
		if err := db.Create(&ib).Error; err != nil {
			b.Fatalf("seed inbound %d: %v", ibIdx, err)
		}
		inboundIDs = append(inboundIDs, ib.Id)
	}

	for i := range traffics {
		traffics[i].InboundId = inboundIDs[0]
	}
	if err := db.CreateInBatches(traffics, 250).Error; err != nil {
		b.Fatalf("seed traffics: %v", err)
	}
	for i := range records {
		for k := range min(inboundsPerClient, len(inboundIDs)) {
			links = append(links, model.ClientInbound{
				ClientId:  records[i].Id,
				InboundId: inboundIDs[k],
			})
		}
	}
	if err := db.CreateInBatches(links, 500).Error; err != nil {
		b.Fatalf("seed client_inbounds: %v", err)
	}
	return inboundIDs, records[0].Email
}

func BenchmarkReadHotPaths(b *testing.B) {
	// 25 inbounds x 3,000 clients (~75,000 client_inbounds rows) mirrors large multi-inbound panels.
	inboundIDs, sampleEmail := seedSyntheticScaleDB(b, 25, 3000, 25)
	clientSvc := &ClientService{}
	inboundSvc := &InboundService{}
	settingSvc := &SettingService{}

	b.Run("ClientList_3000x25", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			rows, err := clientSvc.List()
			if err != nil || len(rows) != 3000 {
				b.Fatalf("List = (%d, %v)", len(rows), err)
			}
		}
	})

	b.Run("ClientWithAttachments_MarshalJSON_3000", func(b *testing.B) {
		rows, err := clientSvc.List()
		if err != nil {
			b.Fatalf("List: %v", err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			if _, err := json.Marshal(rows); err != nil {
				b.Fatalf("Marshal: %v", err)
			}
		}
	})

	b.Run("TunnelAllowedIPs_25NonTunnelInbounds", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			m, err := clientSvc.TunnelAllowedIPsByInbound(inboundSvc, sampleEmail, inboundIDs)
			if err != nil || len(m) != 0 {
				b.Fatalf("TunnelAllowedIPsByInbound = (%v, %v)", m, err)
			}
		}
	})

	b.Run("SettingGetPort_Repeated", func(b *testing.B) {
		if err := settingSvc.SetPort(2053); err != nil {
			b.Fatalf("SetPort: %v", err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			if _, err := settingSvc.GetPort(); err != nil {
				b.Fatalf("GetPort: %v", err)
			}
		}
	})
}
