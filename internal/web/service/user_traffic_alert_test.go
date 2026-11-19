package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/trafficguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func trafficAlertDB(t *testing.T) {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "alerts.db"))
	settings := &SettingService{}
	if err := settings.SetSmtpEnable(true); err != nil {
		t.Fatal(err)
	}
	if err := settings.SetSmtpEnabledEvents("user.rate.high,user.traffic.high"); err != nil {
		t.Fatal(err)
	}
}

func alertState(t *testing.T, email string) trafficguard.State {
	t.Helper()
	var row model.UserTrafficAlertState
	if err := database.GetDB().Where("email = ?", email).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return trafficguard.State{}
		}
		t.Fatal(err)
	}
	var state trafficguard.State
	if err := json.Unmarshal([]byte(row.Payload), &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestUserTrafficAlertsBaselineZeroGapAndUserIsolation(t *testing.T) {
	trafficAlertDB(t)
	var svc UserTrafficAlertService
	epoch := new(xray.Process)
	now := time.Unix(1700000000, 0)
	observe := func(seconds int, a, b int64) {
		t.Helper()
		if err := svc.Observe(now.Add(time.Duration(seconds)*time.Second), epoch, []*xray.ClientTraffic{{Email: "alice", Down: a}, {Email: "bob", Down: b}}); err != nil {
			t.Fatal(err)
		}
	}
	observe(0, 99_000_000_000, 0) // lifetime-like first input must be discarded
	for i := 1; i <= 12; i++ {
		observe(i*5, 40_000_000, 35_000_000)
	}
	if s := alertState(t, "alice"); !s.RateNotified {
		t.Fatal("sustained high user did not alert")
	}
	if s := alertState(t, "bob"); s.RateNotified {
		t.Fatal("threshold equality or another user caused an alert")
	}
	observe(65, 0, 0)
	if s := alertState(t, "alice"); s.RateSince != 0 {
		t.Fatal("zero interval did not reset rate")
	}
	for i := 14; i <= 24; i++ {
		observe(i*5, 40_000_000, 0)
	}
	observe(140, 900_000_000, 0) // 20s gap: entire untrustworthy interval discarded
	observe(145, 40_000_000, 0)
	if s := alertState(t, "alice"); s.RateSince != now.Add(140*time.Second).UnixMilli() {
		t.Fatal("gap bridged old rate streak")
	}
	var sum int64
	if err := database.GetDB().Model(&model.UserTrafficAlertSample{}).Select("COALESCE(SUM(bytes),0)").Scan(&sum).Error; err != nil {
		t.Fatal(err)
	}
	if sum >= 99_000_000_000 {
		t.Fatal("baseline entered rolling history")
	}
}

func TestUserTrafficAlertsRollingBoundaryAndRestart(t *testing.T) {
	trafficAlertDB(t)
	var svc UserTrafficAlertService
	epoch := new(xray.Process)
	now := time.Unix(1700000000, 0)
	tick := func(s *UserTrafficAlertService, sec int, bytes int64) {
		t.Helper()
		if err := s.Observe(now.Add(time.Duration(sec)*time.Second), epoch, []*xray.ClientTraffic{{Email: "alice", Up: bytes}}); err != nil {
			t.Fatal(err)
		}
	}
	tick(&svc, 0, 99_000_000_000)
	tick(&svc, 5, 5_000_000_001)
	before := alertState(t, "alice")
	if !before.WindowNotified {
		t.Fatal("rolling threshold did not fire")
	}
	var restored UserTrafficAlertService
	tick(&restored, 10, 99_000_000_000) // skip startup input again
	tick(&restored, 15, 0)
	after := alertState(t, "alice")
	if after.LastWindowAlert != before.LastWindowAlert || !after.WindowNotified {
		t.Fatal("restart duplicated window alert or lost persisted episode")
	}
	// Core replacement also discards its first input and restarts sustained tracking.
	epoch = new(xray.Process)
	tick(&restored, 20, 99_000_000_000)
	tick(&restored, 25, 0)
	// Jump forward, baseline, then observe a fresh zero interval. The old [0,5]
	// sample only partially overlaps the 30m window and must be wholly excluded.
	tick(&restored, 1797, 0)
	tick(&restored, 1802, 0)
	if s := alertState(t, "alice"); s.WindowNotified {
		t.Fatal("sample outside window counted by end-time alone")
	}
	var sum int64
	database.GetDB().Model(&model.UserTrafficAlertSample{}).Select("COALESCE(SUM(bytes),0)").Scan(&sum)
	if sum != 5_000_000_001 {
		t.Fatalf("startup/core-change bytes were admitted: %d", sum)
	}
}

func TestUserTrafficAlertsDisabledDoesNotCollect(t *testing.T) {
	trafficAlertDB(t)
	settings := &SettingService{}
	if err := settings.SetSmtpEnabledEvents("cpu.high"); err != nil {
		t.Fatal(err)
	}
	var svc UserTrafficAlertService
	now := time.Unix(1700000000, 0)
	for i := 0; i < 15; i++ {
		if err := svc.Observe(now.Add(time.Duration(i)*5*time.Second), nil, []*xray.ClientTraffic{{Email: "alice", Down: 10_000_000_000}}); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	database.GetDB().Model(&model.UserTrafficAlertSample{}).Count(&count)
	if count != 0 {
		t.Fatal("disabled rule collected samples")
	}
}

func TestUserTrafficSettingsRoundTripPreservesCPUAndSMTP(t *testing.T) {
	trafficAlertDB(t)
	settings := &SettingService{}
	if err := settings.SetSmtpPassword("test-password-never-send"); err != nil {
		t.Fatal(err)
	}
	view, err := settings.GetAllSettingView()
	if err != nil {
		t.Fatal(err)
	}
	if view.SmtpUserRateMB != 7 || view.SmtpUserWindowMinutes != 30 {
		t.Fatal("missing defaults")
	}
	view.SmtpEnabledEvents = "cpu.high,user.rate.high,user.traffic.high"
	view.SmtpUserRateMB, view.SmtpUserWindowMinutes = 9, 45
	if err := settings.UpdateAllSetting(&view.AllSetting, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	got, err := settings.GetAllSettingView()
	if err != nil {
		t.Fatal(err)
	}
	if got.SmtpUserRateMB != 9 || got.SmtpUserWindowMinutes != 45 || got.SmtpCpu != 80 || !got.HasSmtpPassword || got.SmtpPassword != "" {
		t.Fatal("settings or redacted SMTP credential were not preserved")
	}
}

func TestUserTrafficCleanupWhileDisabled(t *testing.T) {
	trafficAlertDB(t)
	now := time.Unix(1700000000, 0)
	db := database.GetDB()
	old := model.UserTrafficAlertSample{Email: "old", StartMS: now.Add(-25 * time.Hour).UnixMilli(), EndMS: now.Add(-25 * time.Hour).UnixMilli(), Bytes: 1}
	if err := db.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&SettingService{}).SetSmtpEnable(false); err != nil {
		t.Fatal(err)
	}
	var svc UserTrafficAlertService
	if err := svc.Observe(now, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.Observe(now.Add(5*time.Second), nil, nil); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.UserTrafficAlertSample{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("disabled mail left expired samples behind")
	}
}

func TestUserTrafficBatchQueriesAndIdleWriteSuppression(t *testing.T) {
	trafficAlertDB(t)
	var svc UserTrafficAlertService
	now := time.Unix(1700000000, 0)
	const users = 1201
	deltas := make([]*xray.ClientTraffic, users)
	for i := range deltas {
		deltas[i] = &xray.ClientTraffic{Email: fmt.Sprintf("client-%d", i), Down: 100}
	}
	for i := 0; i < 3; i++ {
		if err := svc.Observe(now.Add(time.Duration(i)*5*time.Second), nil, deltas); err != nil {
			t.Fatal(err)
		}
	}
	var samples, states int64
	db := database.GetDB()
	if err := db.Model(&model.UserTrafficAlertSample{}).Count(&samples).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.UserTrafficAlertState{}).Count(&states).Error; err != nil {
		t.Fatal(err)
	}
	if samples != 2*users || states != 0 {
		t.Fatalf("samples=%d states=%d; want %d and no idle baseline rows", samples, states, 2*users)
	}
}

func BenchmarkUserTrafficAlertPoll(b *testing.B) {
	dbtest.InitDB(b, filepath.Join(b.TempDir(), "alerts.db"))
	settings := &SettingService{}
	if err := settings.SetSmtpEnable(true); err != nil {
		b.Fatal(err)
	}
	if err := settings.SetSmtpEnabledEvents("user.rate.high,user.traffic.high"); err != nil {
		b.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	const users = 1000
	deltas := make([]*xray.ClientTraffic, users)
	rows := make([]model.UserTrafficAlertSample, 0, users*360)
	for i := range deltas {
		email := fmt.Sprintf("bench-%d", i)
		deltas[i] = &xray.ClientTraffic{Email: email, Down: 1_000_000}
		for sample := int64(0); sample < 360; sample++ {
			end := now.UnixMilli() - sample*5000
			rows = append(rows, model.UserTrafficAlertSample{Email: email, StartMS: end - 5000, EndMS: end, Bytes: 1_000_000})
		}
	}
	if err := database.GetDB().CreateInBatches(rows, 100).Error; err != nil {
		b.Fatal(err)
	}
	var svc UserTrafficAlertService
	if err := svc.Observe(now, nil, nil); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := svc.Observe(now.Add(time.Duration(i+1)*5*time.Second), nil, deltas); err != nil {
			b.Fatal(err)
		}
	}
}
