// Package job provides background job implementations for the 3x-ui panel.
package job

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// CollectNodeStatsJob collects traffic and online clients statistics from all nodes.
type CollectNodeStatsJob struct {
	nodeService service.NodeService
}

// NewCollectNodeStatsJob creates a new CollectNodeStatsJob instance.
func NewCollectNodeStatsJob() *CollectNodeStatsJob {
	return &CollectNodeStatsJob{
		nodeService: service.NodeService{},
	}
}

// Run executes the job to collect statistics from all nodes.
func (j *CollectNodeStatsJob) Run() {
	logger.Debug("Starting node stats collection job")
	
	if err := j.nodeService.CollectNodeStats(); err != nil {
		logger.Errorf("Failed to collect node stats: %v", err)
		return
	}
	
	logger.Debug("Node stats collection job completed successfully")
}
