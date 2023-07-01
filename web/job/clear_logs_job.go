package job

import (
	"os"
	"x-ui/logger"
	"x-ui/xray"
)

type ClearLogsJob struct{}

func NewClearLogsJob() *ClearLogsJob {
	return new(ClearLogsJob)
}

// Here Run is an interface method of the Job interface
func (j *ClearLogsJob) Run() {
	logFiles := []string{xray.GetIPLimitLogPath(), xray.GetIPLimitBannedLogPath(), xray.GetAccessPersistentLogPath()}

	// clear log files
	for i := 0; i < len(logFiles); i++ {
		if err := os.Truncate(logFiles[i], 0); err != nil {
			logger.Warning("clear logs job err:", err)
		}
	}
}
