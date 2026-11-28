package service

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func initInboundFeaturesTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
	prevMgr := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{
		APIPort:        func() int { return 0 },
		SetNeedRestart: func() {},
	}))
	t.Cleanup(func() { runtime.SetManager(prevMgr) })
}

// orderFeatureInbound inserts a minimal VLESS inbound the service accepts;
// the tag mirrors the auto-tag so the unique tag column stays valid.
func orderFeatureInbound(t *testing.T, user int, port int, remark string, subSortIndex int) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		UserId:         user,
		Enable:         false,
		Remark:         remark,
		Port:           port,
		Protocol:       model.VLESS,
		Tag:            fmt.Sprintf("in-%d-tcp", port),
		Settings:       `{"clients":[],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
		Sniffing:       `{}`,
		SubSortIndex:   subSortIndex,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	return ib
}

// TestGetInboundsSlimRespectsInboundListOrder checks the configured order
// reaches the slim endpoint for local and node-adopted inbounds alike.
func TestGetInboundsSlimRespectsInboundListOrder(t *testing.T) {
	initInboundFeaturesTestDB(t)
	svc := &InboundService{}
	setting := &SettingService{}

	// Each mode's expected order differs from id order and from every other mode.
	a := orderFeatureInbound(t, 1, 8443, "alpha", 3)
	b := orderFeatureInbound(t, 1, 9443, "zeta", 1)
	c := orderFeatureInbound(t, 1, 443, "beta", 2)

	ids := func(t *testing.T) []int {
		t.Helper()
		list, err := svc.GetInboundsSlim(1)
		if err != nil {
			t.Fatalf("GetInboundsSlim: %v", err)
		}
		out := make([]int, 0, len(list))
		for _, ib := range list {
			out = append(out, ib.Id)
		}
		return out
	}

	cases := []struct {
		name string
		want []int
	}{
		{name: "id", want: []int{a.Id, b.Id, c.Id}},
		{name: "port", want: []int{c.Id, a.Id, b.Id}},
		{name: "remark", want: []int{a.Id, c.Id, b.Id}},
		{name: "subSortIndex", want: []int{b.Id, c.Id, a.Id}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := setting.setString("inboundListOrder", tc.name); err != nil {
				t.Fatalf("set order: %v", err)
			}
			if got := ids(t); !slices.Equal(got, tc.want) {
				t.Fatalf("%s order = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestUpdateAllSettingInboundListOrder rejects an invalid order mode at save
// time so a bad value can never corrupt the list query.
func TestUpdateAllSettingInboundListOrder(t *testing.T) {
	initInboundFeaturesTestDB(t)
	svc := &SettingService{}
	all, err := svc.GetAllSetting()
	if err != nil {
		t.Fatalf("GetAllSetting: %v", err)
	}
	all.InboundListOrder = "buddy-sort"
	if err := svc.UpdateAllSetting(all, SecretClears{}); err == nil ||
		err.Error() != "inbound list order must be id, remark, port, protocol, or subSortIndex" {
		t.Fatalf("invalid order: err = %v, want the exact validation error", err)
	}
	all.InboundListOrder = ""
	if err := svc.UpdateAllSetting(all, SecretClears{}); err != nil {
		t.Fatalf("blank order must normalize, got %v", err)
	}
	if got, _ := svc.GetInboundListOrder(); got != "id" {
		t.Fatalf("GetInboundListOrder = %q, want id", got)
	}
}
