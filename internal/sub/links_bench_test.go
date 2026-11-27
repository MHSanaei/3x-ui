package sub

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestGetAllClientLinksBatchMatchesPerInbound(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "links_parity.db"))
	service.InvalidateAllInboundMeta()
	service.InvalidateSettingsCache()
	db := database.GetDB()
	provider := NewLinkProvider()
	service.RegisterSubLinkProvider(provider)

	client := model.Client{
		Email:      "multi@example.com",
		ID:         "11111111-2222-4333-8444-555555555555",
		Password:   "secret-pw-123",
		SubID:      "sub-multi",
		Flow:       "xtls-rprx-vision",
		PrivateKey: "cHJpdmF0ZS1rZXktMzItYnl0ZXMtcGFkZGluZz09PT0=",
		PublicKey:  "cHVibGljLWtleS0zMi1ieXRlcy1wYWRkaW5nPT09PT0=",
		AllowedIPs: []string{"10.77.0.2/32"},
		Enable:     true,
	}
	rec := model.ClientRecord{
		Email:    client.Email,
		UUID:     client.ID,
		Password: client.Password,
		SubID:    client.SubID,
		Enable:   true,
	}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}

	vlessClient := client
	ssClient := client
	ssClient.Flow = ""
	tuicClient := client
	tuicClient.Flow = ""
	wgClient := client
	wgClient.Flow = ""

	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(b)
	}
	inbounds := []model.Inbound{
		{
			UserId: 1, Remark: "vless-1", Enable: true, Port: 22001, Protocol: model.VLESS,
			Settings:       enc(map[string]any{"decryption": "none", "clients": []model.Client{vlessClient}}),
			StreamSettings: `{"network":"tcp","security":"none"}`, Tag: "in-vless-1",
		},
		{
			UserId: 1, Remark: "ss-1", Enable: true, Port: 22002, Protocol: model.Shadowsocks,
			Settings:       enc(map[string]any{"method": "chacha20-ietf-poly1305", "clients": []model.Client{ssClient}}),
			StreamSettings: `{"network":"tcp","security":"none"}`, Tag: "in-ss-1",
		},
		{
			UserId: 1, Remark: "tuic-1", Enable: true, Port: 22003, Protocol: model.TUIC,
			Settings:       enc(map[string]any{"server": map[string]any{"certificate": "/c", "private_key": "/k", "congestion_control": "bbr", "sni": "tuic.example.com", "alpn": []string{"h3"}}, "clients": []model.Client{tuicClient}}),
			StreamSettings: `{}`, Tag: "in-tuic-1",
		},
		{
			UserId: 1, Remark: "wg-1", Enable: true, Port: 22004, Protocol: model.WireGuard,
			Settings:       enc(map[string]any{"secretKey": "c2VydmVyLXByaXYtMzItYnl0ZXMtcGFkZGluZz09PT0=", "clients": []model.Client{wgClient}}),
			StreamSettings: `{}`, Tag: "in-wg-1",
		},
	}
	var want []string
	for i := range inbounds {
		if err := db.Create(&inbounds[i]).Error; err != nil {
			t.Fatalf("create inbound %d: %v", i, err)
		}
		flow := ""
		if inbounds[i].Protocol == model.VLESS {
			flow = client.Flow
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: inbounds[i].Id, FlowOverride: flow}).Error; err != nil {
			t.Fatalf("create link %d: %v", i, err)
		}
		want = append(want, provider.LinksForClient("example.com", &inbounds[i], client.Email)...)
	}

	inboundSvc := &service.InboundService{}
	got, err := inboundSvc.GetAllClientLinks("example.com", client.Email)
	if err != nil {
		t.Fatalf("GetAllClientLinks: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("GetAllClientLinks mismatch:\ngot:  %v\nwant: %v", got, want)
	}

	// Second call hits warm InboundMeta cache and only loads the single WireGuard row.
	var inboundQueries int64
	cb := "test:links_inbound_q"
	_ = db.Callback().Query().After("gorm:query").Register(cb, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "inbounds" {
			atomic.AddInt64(&inboundQueries, 1)
		}
	})
	defer func() { _ = db.Callback().Query().Remove(cb) }()

	got2, err := inboundSvc.GetAllClientLinks("example.com", client.Email)
	if err != nil || !slices.Equal(got2, want) {
		t.Fatalf("warm GetAllClientLinks = (%v, %v), want %v", got2, err, want)
	}
	if q := atomic.LoadInt64(&inboundQueries); q != 1 {
		t.Fatalf("warm GetAllClientLinks issued %d inbounds queries (expected 1 for WG only)", q)
	}
}

func BenchmarkGetAllClientLinks_25Inbounds(b *testing.B) {
	dbtest.InitDB(b, filepath.Join(b.TempDir(), "sub_bench.db"))
	db := database.GetDB()
	service.RegisterSubLinkProvider(NewLinkProvider())

	const numInbounds = 25
	const clientsPerInbound = 1000
	clients := make([]model.Client, clientsPerInbound)
	for i := range clientsPerInbound {
		clients[i] = model.Client{
			Email:    fmt.Sprintf("user-%04d@example.com", i),
			ID:       fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1),
			Password: fmt.Sprintf("pw-%012d", i+1),
			SubID:    fmt.Sprintf("sub-%04d", i),
			Enable:   true,
		}
	}
	rec := model.ClientRecord{
		Email:    clients[0].Email,
		UUID:     clients[0].ID,
		Password: clients[0].Password,
		SubID:    clients[0].SubID,
		Enable:   true,
	}
	if err := db.Create(&rec).Error; err != nil {
		b.Fatalf("seed record: %v", err)
	}
	for ibIdx := range numInbounds {
		proto := model.Shadowsocks
		raw := map[string]any{"method": "chacha20-ietf-poly1305", "clients": clients}
		if ibIdx == 0 {
			proto = model.VLESS
			raw = map[string]any{"decryption": "none", "clients": clients}
		}
		enc, _ := json.Marshal(raw)
		ib := model.Inbound{
			UserId:         1,
			Remark:         fmt.Sprintf("ib-%02d", ibIdx),
			Enable:         true,
			Port:           21000 + ibIdx,
			Protocol:       proto,
			Settings:       string(enc),
			StreamSettings: `{"network":"tcp","security":"none"}`,
			Tag:            fmt.Sprintf("inbound-%d", 21000+ibIdx),
		}
		if err := db.Create(&ib).Error; err != nil {
			b.Fatalf("seed inbound: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
			b.Fatalf("seed link: %v", err)
		}
	}

	inboundSvc := &service.InboundService{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		links, err := inboundSvc.GetAllClientLinks("example.com", rec.Email)
		if err != nil || len(links) != numInbounds {
			b.Fatalf("GetAllClientLinks = (%d, %v)", len(links), err)
		}
	}
}
