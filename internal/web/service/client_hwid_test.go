package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initClientHwidTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func seedHwidClient(t *testing.T, limit int) *model.ClientRecord {
	t.Helper()
	rec := &model.ClientRecord{
		Email:     "hwid@example.com",
		SubID:     "sub-hwid",
		UUID:      "11111111-2222-4333-8444-555555555555",
		Enable:    true,
		LimitHwid: limit,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return rec
}

func TestClientHwidGate(t *testing.T) {
	initClientHwidTestDB(t)
	svc := &ClientService{}

	seedHwidClient(t, 0)
	res, err := svc.EnforceHwidForSubID("sub-hwid", HwidRequest{})
	if err != nil {
		t.Fatalf("no-limit gate: %v", err)
	}
	if !res.Allowed || res.Active {
		t.Fatalf("no limit should allow missing HWID without active headers: %+v", res)
	}
}

func TestClientHwidGateRegistersAndBlocks(t *testing.T) {
	initClientHwidTestDB(t)
	svc := &ClientService{}
	rec := seedHwidClient(t, 2)

	res, err := svc.EnforceHwidForSubID(rec.SubID, HwidRequest{})
	if err != nil {
		t.Fatalf("missing HWID gate: %v", err)
	}
	if res.Allowed || !res.Active || !res.NotSupported {
		t.Fatalf("missing HWID should be denied as not supported: %+v", res)
	}

	firstRaw := "device-one"
	for _, raw := range []string{firstRaw, "device-two"} {
		res, err = svc.EnforceHwidForSubID(rec.SubID, HwidRequest{
			Hwid:        raw,
			UserAgent:   "Happ/1.0",
			DeviceOS:    "android",
			OsVersion:   "15",
			DeviceModel: raw + "-model",
		})
		if err != nil {
			t.Fatalf("register %s: %v", raw, err)
		}
		if !res.Allowed {
			t.Fatalf("register %s denied: %+v", raw, res)
		}
	}

	res, err = svc.EnforceHwidForSubID(rec.SubID, HwidRequest{Hwid: "device-three"})
	if err != nil {
		t.Fatalf("third HWID gate: %v", err)
	}
	if res.Allowed || !res.MaxDevicesReached || !res.LimitReached {
		t.Fatalf("third unique HWID should be denied after limit: %+v", res)
	}

	res, err = svc.EnforceHwidForSubID(rec.SubID, HwidRequest{
		Hwid:        firstRaw,
		UserAgent:   "Karing/2.0",
		DeviceOS:    "ios",
		OsVersion:   "18",
		DeviceModel: "updated-model",
	})
	if err != nil {
		t.Fatalf("existing HWID after full limit: %v", err)
	}
	if !res.Allowed || !res.LimitReached {
		t.Fatalf("existing registered HWID should pass after limit: %+v", res)
	}

	var hashes []string
	if err := database.GetDB().Model(&model.ClientHwid{}).Pluck("hwid_hash", &hashes).Error; err != nil {
		t.Fatalf("pluck hashes: %v", err)
	}
	if len(hashes) != 2 {
		t.Fatalf("stored HWIDs = %d, want 2", len(hashes))
	}
	for _, h := range hashes {
		if h == firstRaw || h == "device-two" || len(h) != 64 {
			t.Fatalf("raw HWID leaked or invalid hash stored: %q", h)
		}
	}

	list, err := svc.ListClientHwids(rec.Email)
	if err != nil {
		t.Fatalf("list HWIDs: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list count = %d, want 2", len(list))
	}
	foundUpdated := false
	for _, row := range list {
		if row.DeviceModel == "updated-model" && row.UserAgent == "Karing/2.0" && row.DeviceOS == "ios" && row.OsVersion == "18" {
			foundUpdated = true
		}
	}
	if !foundUpdated {
		t.Fatalf("updated HWID metadata missing: %#v", list)
	}

	if err := svc.setClientLimitHwidByEmail(nil, rec.Email, 1); err != nil {
		t.Fatalf("lower limit: %v", err)
	}
	var count int64
	if err := database.GetDB().Model(&model.ClientHwid{}).Where("client_id = ?", rec.Id).Count(&count).Error; err != nil {
		t.Fatalf("count after trim: %v", err)
	}
	if count != 1 {
		t.Fatalf("lowered limit should trim stored HWIDs to 1, got %d", count)
	}

	if err := svc.ClearClientHwids(rec.Email); err != nil {
		t.Fatalf("clear HWIDs: %v", err)
	}
	if err := database.GetDB().Model(&model.ClientHwid{}).Where("client_id = ?", rec.Id).Count(&count).Error; err != nil {
		t.Fatalf("count after clear: %v", err)
	}
	if count != 0 {
		t.Fatalf("clear should remove all HWIDs, got %d", count)
	}
}
