package service

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestXrayTrafficBatchRetriesDeferredCommitFailurePostgres(t *testing.T) {
	cleanup, err := testpg.IsolatePackage("xray_traffic_commit")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	db := durablePostgresDB(t)
	inbound := model.Inbound{Tag: "deferred-in", Protocol: model.VLESS, Enable: true, Settings: "{}"}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	client := xray.ClientTraffic{InboundId: inbound.Id, Email: "deferred@example.test", Enable: true}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	const batchID = "deferred-commit"
	const outboundTag = "deferred-out"
	tags := []*xray.Traffic{
		{Tag: inbound.Tag, IsInbound: true, Up: 19},
		{Tag: outboundTag, IsOutbound: true, Up: 19},
	}
	clients := []*xray.ClientTraffic{{Email: client.Email, Up: 19}}
	add := func() error {
		_, _, err := (&InboundService{}).AddXrayTrafficBatch(batchID, tags, clients)
		return err
	}
	assertTotals := func(want int64) {
		t.Helper()
		var storedInbound model.Inbound
		var storedClient xray.ClientTraffic
		var outbounds []model.OutboundTraffics
		var receipts []model.XrayTrafficReceipt
		for _, err := range []error{
			db.First(&storedInbound, inbound.Id).Error,
			db.First(&storedClient, client.Id).Error,
			db.Where("tag = ?", outboundTag).Find(&outbounds).Error,
			db.Find(&receipts).Error,
		} {
			if err != nil {
				t.Fatal(err)
			}
		}
		if storedInbound.Up != want || storedClient.Up != want {
			t.Errorf("inbound/client up = %d/%d, want %d/%d", storedInbound.Up, storedClient.Up, want, want)
		}
		if want == 0 {
			if len(outbounds) != 0 || len(receipts) != 0 {
				t.Fatalf("failed commit persisted outbound or receipt: %v, %v", outbounds, receipts)
			}
			return
		}
		if len(outbounds) != 1 || outbounds[0].Up != want || len(receipts) != 1 || receipts[0].BatchID != batchID {
			t.Fatalf("expected one outbound with up=%d and one receipt: %v, %v", want, outbounds, receipts)
		}
	}

	if !t.Run("deferred foreign key rolls back whole sample", func(t *testing.T) {
		installDeferredCommitFailure(t, db, "create", "test:xray_traffic_commit", "outbound_traffics", "xray_sample_parent", "xray_sample_child")
		var pgErr *pgconn.PgError
		if err := add(); !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Fatalf("accounting returned %v, want a deferred PostgreSQL foreign-key failure", err)
		}
	}) {
		return
	}
	assertTotals(0)
	if err := add(); err != nil {
		t.Fatal(err)
	}
	assertTotals(19)
	if err := add(); err != nil {
		t.Fatal(err)
	}
	assertTotals(19)
}
