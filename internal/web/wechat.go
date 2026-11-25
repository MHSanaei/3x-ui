package web

import "github.com/mhsanaei/3x-ui/v3/internal/web/job"

func (s *Server) reloadWechatJobs() {
	s.wechatScheduleMu.Lock()
	defer s.wechatScheduleMu.Unlock()
	if s.cron == nil {
		return
	}
	if s.wechatScheduler != s.cron {
		s.wechatScheduler = s.cron
		s.wechatNotifyEntryID, s.cpuAlarmEntryID, s.memoryAlarmEntryID = 0, 0, 0
		s.wechatReportSpec = ""
	}
	enabled, enableErr := s.settingService.GetWechatEnable()
	reports, reportErr := s.settingService.GetWechatReportEnable()
	spec, specErr := s.settingService.GetWechatRunTime()
	wanted := enableErr == nil && reportErr == nil && specErr == nil && enabled && reports
	if s.wechatNotifyEntryID != 0 && (!wanted || spec != s.wechatReportSpec) {
		s.cron.Remove(s.wechatNotifyEntryID)
		s.wechatNotifyEntryID = 0
		s.wechatReportSpec = ""
	}
	if wanted && s.wechatNotifyEntryID == 0 {
		s.wechatNotifyEntryID = scheduleSettingJob(s.cron, "WeChat notify", spec, "@daily", job.NewWechatNotifyJob(s.wechatService))
		s.wechatReportSpec = spec
	}
	if s.cpuAlarmWanted() {
		if s.cpuAlarmEntryID == 0 {
			s.cpuAlarmEntryID, _ = s.cron.AddJob(cadenceCPUAlarm, job.NewCheckCpuJob())
		}
	} else if s.cpuAlarmEntryID != 0 {
		s.cron.Remove(s.cpuAlarmEntryID)
		s.cpuAlarmEntryID = 0
	}
	if s.memoryAlarmWanted() {
		if s.memoryAlarmEntryID == 0 {
			s.memoryAlarmEntryID, _ = s.cron.AddJob(cadenceMemoryAlarm, job.NewCheckMemJob())
		}
	} else if s.memoryAlarmEntryID != 0 {
		s.cron.Remove(s.memoryAlarmEntryID)
		s.memoryAlarmEntryID = 0
	}
}
