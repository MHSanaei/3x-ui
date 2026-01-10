// Package service provides Node management service for multi-node architecture.
package service

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/xray"
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
// Only updates fields that are provided (non-empty for strings, non-zero for integers).
func (s *NodeService) UpdateNode(node *model.Node) error {
	db := database.GetDB()
	
	// Get existing node to preserve fields that are not being updated
	existingNode, err := s.GetNode(node.Id)
	if err != nil {
		return fmt.Errorf("failed to get existing node: %w", err)
	}
	
	// Update only provided fields
	updates := make(map[string]interface{})
	
	if node.Name != "" {
		updates["name"] = node.Name
	}
	
	if node.Address != "" {
		updates["address"] = node.Address
	}
	
	if node.ApiKey != "" {
		updates["api_key"] = node.ApiKey
	}
	
	// Update TLS settings if provided
	updates["use_tls"] = node.UseTLS
	if node.CertPath != "" {
		updates["cert_path"] = node.CertPath
	}
	if node.KeyPath != "" {
		updates["key_path"] = node.KeyPath
	}
	updates["insecure_tls"] = node.InsecureTLS
	
	// Update status and last_check if provided (these are usually set by health checks, not user edits)
	if node.Status != "" && node.Status != existingNode.Status {
		updates["status"] = node.Status
	}
	
	if node.LastCheck > 0 && node.LastCheck != existingNode.LastCheck {
		updates["last_check"] = node.LastCheck
	}
	
	// If no fields to update, return early
	if len(updates) == 0 {
		return nil
	}
	
	// Update only the specified fields
	return db.Model(existingNode).Updates(updates).Error
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

// createHTTPClient creates an HTTP client configured for the node's TLS settings.
func (s *NodeService) createHTTPClient(node *model.Node, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: node.InsecureTLS,
		},
	}

	// If custom certificates are provided, load them
	if node.UseTLS && node.CertPath != "" {
		// Load custom CA certificate
		cert, err := os.ReadFile(node.CertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read certificate file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(cert) {
			return nil, fmt.Errorf("failed to parse certificate")
		}

		transport.TLSClientConfig.RootCAs = caCertPool
		transport.TLSClientConfig.InsecureSkipVerify = false // Use custom CA
	}

	// If custom key is provided, load client certificate
	if node.UseTLS && node.KeyPath != "" && node.CertPath != "" {
		// Load client certificate (cert + key)
		clientCert, err := tls.LoadX509KeyPair(node.CertPath, node.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		transport.TLSClientConfig.Certificates = []tls.Certificate{clientCert}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

// CheckNodeStatus performs a health check on a given node.
func (s *NodeService) CheckNodeStatus(node *model.Node) (string, error) {
	client, err := s.createHTTPClient(node, 5*time.Second)
	if err != nil {
		return "error", err
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

// NodeStatsResponse represents the response from node stats API.
type NodeStatsResponse struct {
	Traffic       []*NodeTraffic       `json:"traffic"`
	ClientTraffic []*NodeClientTraffic `json:"clientTraffic"`
	OnlineClients []string              `json:"onlineClients"`
}

// NodeTraffic represents traffic statistics from a node.
type NodeTraffic struct {
	IsInbound  bool   `json:"isInbound"`
	IsOutbound bool   `json:"isOutbound"`
	Tag        string `json:"tag"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
}

// NodeClientTraffic represents client traffic statistics from a node.
type NodeClientTraffic struct {
	Email string `json:"email"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
}

// GetNodeStats retrieves traffic and online clients statistics from a node.
func (s *NodeService) GetNodeStats(node *model.Node, reset bool) (*NodeStatsResponse, error) {
	client, err := s.createHTTPClient(node, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/stats", node.Address)
	if reset {
		url += "?reset=true"
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+node.ApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request node stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("node returned status code %d: %s", resp.StatusCode, string(body))
	}

	var stats NodeStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &stats, nil
}

// CollectNodeStats collects statistics from all nodes and aggregates them into the database.
// This should be called periodically (e.g., via cron job).
func (s *NodeService) CollectNodeStats() error {
	// Check if multi-node mode is enabled
	settingService := SettingService{}
	multiMode, err := settingService.GetMultiNodeMode()
	if err != nil || !multiMode {
		return nil // Skip if multi-node mode is not enabled
	}

	nodes, err := s.GetAllNodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	if len(nodes) == 0 {
		return nil // No nodes to collect stats from
	}

	// Filter nodes: only collect stats from nodes that have assigned inbounds
	nodesWithInbounds := make([]*model.Node, 0)
	for _, node := range nodes {
		inbounds, err := s.GetInboundsForNode(node.Id)
		if err == nil && len(inbounds) > 0 {
			// Only include nodes that have at least one assigned inbound
			nodesWithInbounds = append(nodesWithInbounds, node)
		}
	}

	if len(nodesWithInbounds) == 0 {
		return nil // No nodes with assigned inbounds
	}

	// Import inbound service to aggregate traffic
	inboundService := &InboundService{}

	// Collect stats from nodes with assigned inbounds concurrently
	type nodeStatsResult struct {
		node  *model.Node
		stats *NodeStatsResponse
		err   error
	}

	results := make(chan nodeStatsResult, len(nodesWithInbounds))
	for _, node := range nodesWithInbounds {
		go func(n *model.Node) {
			stats, err := s.GetNodeStats(n, false) // Don't reset counters on collection
			results <- nodeStatsResult{node: n, stats: stats, err: err}
		}(node)
	}

	// Aggregate all traffic
	allTraffics := make([]*xray.Traffic, 0)
	allClientTraffics := make([]*xray.ClientTraffic, 0)
	onlineClientsMap := make(map[string]bool)

	for i := 0; i < len(nodesWithInbounds); i++ {
		result := <-results
		if result.err != nil {
			// Check if error is expected (XRAY not running, 404 for old nodes, etc.)
			errMsg := result.err.Error()
			if strings.Contains(errMsg, "XRAY is not running") || 
			   strings.Contains(errMsg, "status code 404") ||
			   strings.Contains(errMsg, "status code 500") {
				// These are expected errors, log as debug only
				logger.Debugf("Skipping stats collection from node %s (ID: %d): %v", result.node.Name, result.node.Id, result.err)
			} else {
				// Unexpected errors should be logged as warning
				logger.Warningf("Failed to get stats from node %s (ID: %d): %v", result.node.Name, result.node.Id, result.err)
			}
			continue
		}

		if result.stats == nil {
			continue
		}

		// Convert node traffic to xray.Traffic
		for _, nt := range result.stats.Traffic {
			allTraffics = append(allTraffics, &xray.Traffic{
				IsInbound:  nt.IsInbound,
				IsOutbound: nt.IsOutbound,
				Tag:        nt.Tag,
				Up:         nt.Up,
				Down:       nt.Down,
			})
		}

		// Convert node client traffic to xray.ClientTraffic
		for _, nct := range result.stats.ClientTraffic {
			allClientTraffics = append(allClientTraffics, &xray.ClientTraffic{
				Email: nct.Email,
				Up:    nct.Up,
				Down:  nct.Down,
			})
		}

		// Collect online clients
		for _, email := range result.stats.OnlineClients {
			onlineClientsMap[email] = true
		}
	}

	// Aggregate traffic into database
	if len(allTraffics) > 0 || len(allClientTraffics) > 0 {
		_, needRestart := inboundService.AddTraffic(allTraffics, allClientTraffics)
		if needRestart {
			logger.Info("Traffic aggregation triggered client renewal/disabling, restart may be needed")
		}
	}

	logger.Debugf("Collected stats from nodes: %d traffics, %d client traffics, %d online clients",
		len(allTraffics), len(allClientTraffics), len(onlineClientsMap))

	return nil
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
	client, err := s.createHTTPClient(node, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
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

// ReloadNode reloads XRAY on a specific node.
func (s *NodeService) ReloadNode(node *model.Node) error {
	client, err := s.createHTTPClient(node, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/reload", node.Address)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

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

// ForceReloadNode forcefully reloads XRAY on a specific node (even if hung).
func (s *NodeService) ForceReloadNode(node *model.Node) error {
	client, err := s.createHTTPClient(node, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/force-reload", node.Address)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

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

// ReloadAllNodes reloads XRAY on all nodes.
func (s *NodeService) ReloadAllNodes() error {
	nodes, err := s.GetAllNodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	type reloadResult struct {
		node *model.Node
		err  error
	}

	results := make(chan reloadResult, len(nodes))
	for _, node := range nodes {
		go func(n *model.Node) {
			err := s.ForceReloadNode(n) // Use force reload to handle hung nodes
			results <- reloadResult{node: n, err: err}
		}(node)
	}

	var errors []string
	for i := 0; i < len(nodes); i++ {
		result := <-results
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("node %d (%s): %v", result.node.Id, result.node.Name, result.err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to reload some nodes: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ValidateApiKey validates the API key by making a test request to the node.
func (s *NodeService) ValidateApiKey(node *model.Node) error {
	client, err := s.createHTTPClient(node, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
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
	client, err := s.createHTTPClient(node, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
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
