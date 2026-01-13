// Package job provides scheduled background jobs for the 3x-ui panel.
package job

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/websocket"
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
	
	// Use a wait group to wait for all health checks to complete
	var wg sync.WaitGroup
	for _, node := range nodes {
		n := node // Capture loop variable
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := j.nodeService.CheckNodeHealth(n); err != nil {
				logger.Debugf("[Node: %s] Health check failed: %v", n.Name, err)
			} else {
				logger.Debugf("[Node: %s] Status: %s, ResponseTime: %d ms", n.Name, n.Status, n.ResponseTime)
			}
		}()
	}
	
	// Wait for all checks to complete, then broadcast update
	go func() {
		wg.Wait()
		// Get updated nodes with response times
		updatedNodes, err := j.nodeService.GetAllNodes()
		if err != nil {
			logger.Warningf("Failed to get nodes for WebSocket broadcast: %v", err)
			return
		}
		
		// Enrich nodes with assigned inbounds information
		type NodeWithInbounds struct {
			*model.Node
			Inbounds []*model.Inbound `json:"inbounds,omitempty"`
		}
		
		result := make([]NodeWithInbounds, 0, len(updatedNodes))
		for _, node := range updatedNodes {
			inbounds, _ := j.nodeService.GetInboundsForNode(node.Id)
			result = append(result, NodeWithInbounds{
				Node:     node,
				Inbounds: inbounds,
			})
		}
		
		// Broadcast via WebSocket
		websocket.BroadcastNodes(result)
	}()
}
