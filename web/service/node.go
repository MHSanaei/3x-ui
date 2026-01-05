// Package service provides Node management service for multi-node architecture.
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

// NodeService provides business logic for managing nodes in multi-node mode.
type NodeService struct{}

// GetAllNodes retrieves all nodes from the database.
func (s *NodeService) GetAllNodes() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	err := db.Find(&nodes).Error
	return nodes, err
}

// GetNode retrieves a node by ID.
func (s *NodeService) GetNode(id int) (*model.Node, error) {
	db := database.GetDB()
	var node model.Node
	err := db.First(&node, id).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// AddNode creates a new node.
func (s *NodeService) AddNode(node *model.Node) error {
	db := database.GetDB()
	return db.Create(node).Error
}

// UpdateNode updates an existing node.
func (s *NodeService) UpdateNode(node *model.Node) error {
	db := database.GetDB()
	return db.Save(node).Error
}

// DeleteNode deletes a node by ID.
// This will cascade delete all InboundNodeMapping entries for this node.
func (s *NodeService) DeleteNode(id int) error {
	db := database.GetDB()
	
	// Delete all node mappings for this node (cascade delete)
	err := db.Where("node_id = ?", id).Delete(&model.InboundNodeMapping{}).Error
	if err != nil {
		return err
	}
	
	// Delete the node itself
	return db.Delete(&model.Node{}, id).Error
}

// CheckNodeHealth checks if a node is online and updates its status.
func (s *NodeService) CheckNodeHealth(node *model.Node) error {
	status, err := s.CheckNodeStatus(node)
	if err != nil {
		node.Status = "error"
		node.LastCheck = time.Now().Unix()
		s.UpdateNode(node)
		return err
	}
	node.Status = status
	node.LastCheck = time.Now().Unix()
	return s.UpdateNode(node)
}

// CheckNodeStatus performs a health check on a given node.
func (s *NodeService) CheckNodeStatus(node *model.Node) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	url := fmt.Sprintf("%s/health", node.Address)
	resp, err := client.Get(url)
	if err != nil {
		return "offline", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "online", nil
	}
	return "error", fmt.Errorf("node returned status code %d", resp.StatusCode)
}

// CheckAllNodesHealth checks health of all nodes.
func (s *NodeService) CheckAllNodesHealth() {
	nodes, err := s.GetAllNodes()
	if err != nil {
		logger.Errorf("Failed to get nodes for health check: %v", err)
		return
	}

	for _, node := range nodes {
		go s.CheckNodeHealth(node)
	}
}

// GetNodeForInbound returns the node assigned to an inbound, or nil if not assigned.
// Deprecated: Use GetNodesForInbound for multi-node support.
func (s *NodeService) GetNodeForInbound(inboundId int) (*model.Node, error) {
	db := database.GetDB()
	var mapping model.InboundNodeMapping
	err := db.Where("inbound_id = ?", inboundId).First(&mapping).Error
	if err != nil {
		return nil, err // Not found is OK, means inbound is not assigned to any node
	}

	return s.GetNode(mapping.NodeId)
}

// GetNodesForInbound returns all nodes assigned to an inbound.
func (s *NodeService) GetNodesForInbound(inboundId int) ([]*model.Node, error) {
	db := database.GetDB()
	var mappings []model.InboundNodeMapping
	err := db.Where("inbound_id = ?", inboundId).Find(&mappings).Error
	if err != nil {
		return nil, err
	}

	nodes := make([]*model.Node, 0, len(mappings))
	for _, mapping := range mappings {
		node, err := s.GetNode(mapping.NodeId)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// GetInboundsForNode returns all inbounds assigned to a node.
func (s *NodeService) GetInboundsForNode(nodeId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var mappings []model.InboundNodeMapping
	err := db.Where("node_id = ?", nodeId).Find(&mappings).Error
	if err != nil {
		return nil, err
	}

	inbounds := make([]*model.Inbound, 0, len(mappings))
	for _, mapping := range mappings {
		var inbound model.Inbound
		err := db.First(&inbound, mapping.InboundId).Error
		if err == nil {
			inbounds = append(inbounds, &inbound)
		}
	}
	return inbounds, nil
}

// AssignInboundToNode assigns an inbound to a node.
func (s *NodeService) AssignInboundToNode(inboundId, nodeId int) error {
	db := database.GetDB()
	mapping := &model.InboundNodeMapping{
		InboundId: inboundId,
		NodeId:    nodeId,
	}
	return db.Save(mapping).Error
}

// AssignInboundToNodes assigns an inbound to multiple nodes.
func (s *NodeService) AssignInboundToNodes(inboundId int, nodeIds []int) error {
	db := database.GetDB()
	// First, remove all existing assignments
	if err := db.Where("inbound_id = ?", inboundId).Delete(&model.InboundNodeMapping{}).Error; err != nil {
		return err
	}
	
	// Then, create new assignments
	for _, nodeId := range nodeIds {
		if nodeId > 0 {
			mapping := &model.InboundNodeMapping{
				InboundId: inboundId,
				NodeId:    nodeId,
			}
			if err := db.Create(mapping).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// UnassignInboundFromNode removes the assignment of an inbound from its node.
func (s *NodeService) UnassignInboundFromNode(inboundId int) error {
	db := database.GetDB()
	return db.Where("inbound_id = ?", inboundId).Delete(&model.InboundNodeMapping{}).Error
}

// ApplyConfigToNode sends XRAY configuration to a node.
func (s *NodeService) ApplyConfigToNode(node *model.Node, xrayConfig []byte) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	url := fmt.Sprintf("%s/api/v1/apply-config", node.Address)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(xrayConfig))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", node.ApiKey))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("node returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ValidateApiKey validates the API key by making a test request to the node.
func (s *NodeService) ValidateApiKey(node *model.Node) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// First, check if node is reachable via health endpoint
	healthURL := fmt.Sprintf("%s/health", node.Address)
	healthResp, err := client.Get(healthURL)
	if err != nil {
		logger.Errorf("Failed to connect to node %s at %s: %v", node.Address, healthURL, err)
		return fmt.Errorf("failed to connect to node: %v", err)
	}
	healthResp.Body.Close()
	
	if healthResp.StatusCode != http.StatusOK {
		return fmt.Errorf("node health check failed with status %d", healthResp.StatusCode)
	}

	// Try to get node status - this will validate the API key
	url := fmt.Sprintf("%s/api/v1/status", node.Address)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	authHeader := fmt.Sprintf("Bearer %s", node.ApiKey)
	req.Header.Set("Authorization", authHeader)
	
	logger.Debugf("Validating API key for node %s at %s (key: %s)", node.Name, url, node.ApiKey)

	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to connect to node %s: %v", node.Address, err)
		return fmt.Errorf("failed to connect to node: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode == http.StatusUnauthorized {
		logger.Warningf("Invalid API key for node %s (sent: %s): %s", node.Address, authHeader, string(body))
		return fmt.Errorf("invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("Node %s returned status %d: %s", node.Address, resp.StatusCode, string(body))
		return fmt.Errorf("node returned status %d: %s", resp.StatusCode, string(body))
	}

	logger.Debugf("API key validated successfully for node %s", node.Name)
	return nil
}

// GetNodeStatus retrieves the status of a node.
func (s *NodeService) GetNodeStatus(node *model.Node) (map[string]interface{}, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	url := fmt.Sprintf("%s/api/v1/status", node.Address)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", node.ApiKey))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return status, nil
}
