package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/json_util"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/gorm"
)

// OutboundService provides business logic for managing Xray outbound configurations.
// It handles outbound traffic monitoring and statistics.
type OutboundService struct{}

func (s *OutboundService) AddTraffic(traffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (error, bool) {
	var err error
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = s.addOutboundTraffic(tx, traffics)
	if err != nil {
		return err, false
	}

	return nil, false
}

func (s *OutboundService) addOutboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsOutbound {

			var outbound model.OutboundTraffics

			err = tx.Model(&model.OutboundTraffics{}).Where("tag = ?", traffic.Tag).
				FirstOrCreate(&outbound).Error
			if err != nil {
				return err
			}

			outbound.Tag = traffic.Tag
			outbound.Up = outbound.Up + traffic.Up
			outbound.Down = outbound.Down + traffic.Down
			outbound.Total = outbound.Up + outbound.Down

			err = tx.Save(&outbound).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *OutboundService) GetOutboundsTraffic() ([]*model.OutboundTraffics, error) {
	db := database.GetDB()
	var traffics []*model.OutboundTraffics

	err := db.Model(model.OutboundTraffics{}).Find(&traffics).Error
	if err != nil {
		logger.Warning("Error retrieving OutboundTraffics: ", err)
		return nil, err
	}

	return traffics, nil
}

func (s *OutboundService) ResetOutboundTraffic(tag string) error {
	db := database.GetDB()

	whereText := "tag "
	if tag == "-alltags-" {
		whereText += " <> ?"
	} else {
		whereText += " = ?"
	}

	result := db.Model(model.OutboundTraffics{}).
		Where(whereText, tag).
		Updates(map[string]any{"up": 0, "down": 0, "total": 0})

	err := result.Error
	if err != nil {
		return err
	}

	return nil
}

// TestOutboundResult represents the result of testing an outbound
type TestOutboundResult struct {
	Success    bool   `json:"success"`
	Delay      int64  `json:"delay"` // Delay in milliseconds
	Error      string `json:"error,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
}

// TestOutbound tests an outbound by creating a temporary xray instance and measuring response time.
// allOutboundsJSON must be a JSON array of all outbounds; they are copied into the test config unchanged.
// Only the test inbound and a route rule (to the tested outbound tag) are added.
func (s *OutboundService) TestOutbound(outboundJSON string, testURL string, allOutboundsJSON string) (*TestOutboundResult, error) {
	if testURL == "" {
		testURL = "http://www.google.com/gen_204"
	}

	// Parse the outbound being tested to get its tag
	var testOutbound map[string]interface{}
	if err := json.Unmarshal([]byte(outboundJSON), &testOutbound); err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid outbound JSON: %v", err),
		}, nil
	}
	outboundTag, _ := testOutbound["tag"].(string)
	if outboundTag == "" {
		return &TestOutboundResult{
			Success: false,
			Error:   "Outbound has no tag",
		}, nil
	}

	// Use all outbounds when provided; otherwise fall back to single outbound
	var allOutbounds []interface{}
	if allOutboundsJSON != "" {
		if err := json.Unmarshal([]byte(allOutboundsJSON), &allOutbounds); err != nil {
			return &TestOutboundResult{
				Success: false,
				Error:   fmt.Sprintf("Invalid allOutbounds JSON: %v", err),
			}, nil
		}
	}
	if len(allOutbounds) == 0 {
		allOutbounds = []interface{}{testOutbound}
	}

	// Find an available port for test inbound
	testPort, err := findAvailablePort()
	if err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to find available port: %v", err),
		}, nil
	}

	// Copy all outbounds as-is, add only test inbound and route rule
	testConfig := s.createTestConfig(outboundTag, allOutbounds, testPort)

	// Use a temporary config file so the main config.json is never overwritten
	testConfigPath, err := createTestConfigPath()
	if err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to create test config path: %v", err),
		}, nil
	}
	defer os.Remove(testConfigPath) // ensure temp file is removed even if process is not stopped

	// Create temporary xray process with its own config file
	testProcess := xray.NewTestProcess(testConfig, testConfigPath)
	defer func() {
		if testProcess.IsRunning() {
			testProcess.Stop()
			// Give it a moment to clean up
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Start the test process
	if err := testProcess.Start(); err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to start test xray instance: %v", err),
		}, nil
	}

	// Wait a bit for xray to start
	time.Sleep(1 * time.Second)

	// Check if process is still running
	if !testProcess.IsRunning() {
		result := testProcess.GetResult()
		return &TestOutboundResult{
			Success: false,
			Error:   fmt.Sprintf("Xray process exited: %s", result),
		}, nil
	}

	// Test the connection through proxy
	delay, statusCode, err := s.testConnection(testPort, testURL)
	if err != nil {
		return &TestOutboundResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &TestOutboundResult{
		Success:    true,
		Delay:      delay,
		StatusCode: statusCode,
	}, nil
}

// createTestConfig creates a test config by copying all outbounds unchanged and adding
// only the test inbound (SOCKS) and a route rule that sends traffic to the given outbound tag.
func (s *OutboundService) createTestConfig(outboundTag string, allOutbounds []interface{}, testPort int) *xray.Config {
	// Test inbound (SOCKS proxy) - only addition to inbounds
	testInbound := xray.InboundConfig{
		Tag:      "test-inbound",
		Listen:   json_util.RawMessage(`"127.0.0.1"`),
		Port:     testPort,
		Protocol: "socks",
		Settings: json_util.RawMessage(`{"auth":"noauth","udp":true}`),
	}

	// Outbounds: copy all as-is, no tag or structure changes
	outboundsJSON, _ := json.Marshal(allOutbounds)

	// Create routing rule to route all traffic through test outbound
	routingRules := []map[string]interface{}{
		{
			"type":        "field",
			"outboundTag": outboundTag,
			"network":     "tcp,udp",
		},
	}

	routingJSON, _ := json.Marshal(map[string]interface{}{
		"domainStrategy": "AsIs",
		"rules":          routingRules,
	})

	// Create minimal config
	config := &xray.Config{
		LogConfig: json_util.RawMessage(`{
			"loglevel":"info",
			"access":"` + config.GetBinFolderPath() + `/access_tests.log",
			"error":"` + config.GetBinFolderPath() + `/error_tests.log",
			"dnsLog":true
		}`),
		InboundConfigs: []xray.InboundConfig{
			testInbound,
		},
		OutboundConfigs: json_util.RawMessage(string(outboundsJSON)),
		RouterConfig:    json_util.RawMessage(string(routingJSON)),
		Policy:          json_util.RawMessage(`{}`),
		Stats:           json_util.RawMessage(`{}`),
	}

	return config
}

// testConnection tests the connection through the proxy and measures delay
func (s *OutboundService) testConnection(proxyPort int, testURL string) (int64, int, error) {
	// Create SOCKS5 proxy URL
	proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", proxyPort)

	// Parse proxy URL
	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return 0, 0, common.NewErrorf("Invalid proxy URL: %v", err)
	}

	// Create HTTP client with proxy
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
			DialContext: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).DialContext,
		},
	}

	// Measure time
	startTime := time.Now()
	resp, err := client.Get(testURL)
	delay := time.Since(startTime).Milliseconds()

	if err != nil {
		return 0, 0, common.NewErrorf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	return delay, resp.StatusCode, nil
}

// findAvailablePort finds an available port for testing
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// createTestConfigPath returns a unique path for a temporary xray config file in the bin folder.
// The file is not created; the path is reserved by creating and then removing an empty temp file.
func createTestConfigPath() (string, error) {
	tmpFile, err := os.CreateTemp(config.GetBinFolderPath(), "xray_test_*.json")
	if err != nil {
		return "", err
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}
