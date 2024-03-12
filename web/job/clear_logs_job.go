package job

import (
	"io"
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
	logFilesPrev := []string{xray.GetIPLimitBannedPrevLogPath(), xray.GetAccessPersistentPrevLogPath()}

	// clear old previous logs
	for i := 0; i < len(logFilesPrev); i++ {
		if err := os.Truncate(logFilesPrev[i], 0); err != nil {
			logger.Warning("clear logs job err:", err)
		}
	}

	// clear log files and copy to previous logs
	for i := 0; i < len(logFiles); i++ {
		if i > 0 {
			// copy to previous logs
			logFilePrev, err := os.OpenFile(logFilesPrev[i-1], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				logger.Warning("clear logs job err:", err)
			}

			logFile, err := os.Open(logFiles[i])
			if err != nil {
				logger.Warning("clear logs job err:", err)
			}

			_, err = io.Copy(logFilePrev, logFile)
			if err != nil {
				logger.Warning("clear logs job err:", err)
			}

			logFile.Close()
			logFilePrev.Close()
		}

		err := os.Truncate(logFiles[i], 0)
		if err != nil {
			logger.Warning("clear logs job err:", err)
		}
	}
}
