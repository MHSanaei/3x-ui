// Package job provides scheduled background jobs for the 3x-ui panel.
package job

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// CheckNodeHealthJob periodically checks the health of all nodes in multi-node mode.
type CheckNodeHealthJob struct {
	nodeService service.NodeService
}

// NewCheckNodeHealthJob creates a new job for checking node health.
func NewCheckNodeHealthJob() *CheckNodeHealthJob {
	return &CheckNodeHealthJob{
		nodeService: service.NodeService{},
	}
}

// Run executes the health check for all nodes.
func (j *CheckNodeHealthJob) Run() {
	// Check if multi-node mode is enabled
	settingService := service.SettingService{}
	multiMode, err := settingService.GetMultiNodeMode()
	if err != nil || !multiMode {
		return // Skip if multi-node mode is not enabled
	}

	nodes, err := j.nodeService.GetAllNodes()
	if err != nil {
		logger.Errorf("Failed to get nodes for health check: %v", err)
		return
	}

	if len(nodes) == 0 {
		return // No nodes to check
	}

	logger.Debugf("Checking health of %d nodes", len(nodes))
	for _, node := range nodes {
		n := node // Capture loop variable
		go func() {
			if err := j.nodeService.CheckNodeHealth(n); err != nil {
				logger.Debugf("Node %s (%s) health check failed: %v", n.Name, n.Address, err)
			} else {
				logger.Debugf("Node %s (%s) is %s", n.Name, n.Address, n.Status)
			}
		}()
	}
}
