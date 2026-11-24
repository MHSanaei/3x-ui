package web

import (
	"path/filepath"
	"testing"

	"github.com/robfig/cron/v3"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/wechat"
)

func TestWechatSettingsReloadSchedulesReportsAndMetricsWithoutTelegram(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	save := func(key, value string) {
		t.Helper()
		row := model.Setting{Key: key, Value: value}
		if err := database.GetDB().Where("key = ?", key).Assign(model.Setting{Value: value}).FirstOrCreate(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"tgBotEnable", "smtpEnable", "discordBotEnable", "wechatEnable"} {
		save(key, "false")
	}
	s := &Server{cron: cron.New(), wechatService: wechat.NewService(service.SettingService{})}
	s.reloadWechatJobs()
	if got := len(s.cron.Entries()); got != 0 {
		t.Fatalf("disabled notifiers scheduled %d jobs, want zero", got)
	}
	save("wechatEnable", "true")
	save("wechatEnabledEvents", "cpu.high,memory.high")
	save("wechatReportEnable", "true")
	save("wechatRunTime", "@daily")
	s.reloadWechatJobs()
	if got := len(s.cron.Entries()); got != 3 || s.wechatNotifyEntryID == 0 || s.cpuAlarmEntryID == 0 || s.memoryAlarmEntryID == 0 {
		t.Fatalf("newly enabled WeChat scheduled %d jobs, want report plus CPU and memory samplers", got)
	}
	oldReport, oldCPU := s.wechatNotifyEntryID, s.cpuAlarmEntryID
	save("pageSize", "50")
	s.reloadWechatJobs()
	if s.wechatNotifyEntryID != oldReport || s.cpuAlarmEntryID != oldCPU {
		t.Fatal("unrelated settings save restarted the report schedule or metric sampler")
	}
	save("wechatRunTime", "@hourly")
	s.reloadWechatJobs()
	if s.wechatNotifyEntryID == oldReport || s.cpuAlarmEntryID != oldCPU || len(s.cron.Entries()) != 3 {
		t.Fatal("changing report schedule retained the old report or duplicated metric samplers")
	}
	s.cron = cron.New()
	unrelatedID, err := s.cron.AddFunc("@hourly", func() {})
	if err != nil {
		t.Fatal(err)
	}
	s.reloadWechatJobs()
	if len(s.cron.Entries()) != 4 || !s.cron.Entry(unrelatedID).Valid() {
		t.Fatal("scheduler replacement lost metric samplers or removed an unrelated job with a reused ID")
	}
	oldCPU = s.cpuAlarmEntryID
	save("wechatEnable", "false")
	save("smtpEnable", "true")
	save("smtpEnabledEvents", "cpu.high")
	s.reloadWechatJobs()
	if len(s.cron.Entries()) != 2 || !s.cron.Entry(unrelatedID).Valid() || s.cpuAlarmEntryID != oldCPU || s.memoryAlarmEntryID != 0 || s.wechatNotifyEntryID != 0 {
		t.Fatal("disabling WeChat removed another notifier's CPU sampler or retained WeChat-only jobs")
	}
}
