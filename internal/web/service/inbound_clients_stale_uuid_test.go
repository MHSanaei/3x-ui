package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// GetInbounds must enrich ClientStats.UUID from the clients table, not the
// embedded inbound settings JSON, when the two diverge (#6436).
func TestEnrichClientStats_UsesClientsTableUUIDWhenSettingsStale(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	stale := "11111111-1111-1111-1111-111111111111"
	fresh := "22222222-2222-2222-2222-222222222222"

	ib := &model.Inbound{
		UserId: 1, Tag: "stale-uuid-list", Enable: true, Listen: "0.0.0.0", Port: 8443,
		Protocol: model.VLESS, Remark: "stale",
		Settings:       `{"clients":[{"id":"` + stale + `","email":"stale@e","subId":"sub1","enable":true}],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rec := &model.ClientRecord{Email: "stale@e", SubID: "fresh-sub", UUID: fresh, Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("create client_inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: "stale@e", Enable: true}).Error; err != nil {
		t.Fatalf("create client_traffics: %v", err)
	}

	svc := &InboundService{}
	inbounds, err := svc.GetInbounds(1)
	if err != nil {
		t.Fatalf("GetInbounds: %v", err)
	}
	if len(inbounds) != 1 {
		t.Fatalf("inbounds = %d, want 1", len(inbounds))
	}
	if len(inbounds[0].ClientStats) != 1 {
		t.Fatalf("ClientStats = %d, want 1", len(inbounds[0].ClientStats))
	}
	got := inbounds[0].ClientStats[0].UUID
	if got != fresh {
		t.Fatalf("ClientStats.UUID = %q, want fresh %q (settings still had stale %q)", got, fresh, stale)
	}
	if gotSub := inbounds[0].ClientStats[0].SubId; gotSub != "fresh-sub" {
		t.Fatalf("ClientStats.SubId = %q, want fresh-sub", gotSub)
	}
	for _, identity := range []struct{ uuid, sub string }{
		{"33333333-3333-3333-3333-333333333333", "next-sub"},
		{"44444444-4444-4444-4444-444444444444", ""},
	} {
		if err := db.Model(rec).Updates(map[string]any{"uuid": identity.uuid, "sub_id": identity.sub}).Error; err != nil {
			t.Fatal(err)
		}
		inbounds, err = svc.GetInbounds(1)
		if err != nil {
			t.Fatal(err)
		}
		if st := inbounds[0].ClientStats[0]; st.UUID != identity.uuid || st.SubId != identity.sub {
			t.Fatalf("successive read identity = %q/%q, want %q/%q", st.UUID, st.SubId, identity.uuid, identity.sub)
		}
	}
}
