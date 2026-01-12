// Package main is the entry point for the 3x-ui node service (worker).
// This service runs XRAY Core and provides a REST API for the master panel to manage it.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/node/api"
	nodeConfig "github.com/mhsanaei/3x-ui/v2/node/config"
	nodeLogs "github.com/mhsanaei/3x-ui/v2/node/logs"
	"github.com/mhsanaei/3x-ui/v2/node/xray"
	"github.com/op/go-logging"
)


func main() {
	var port int
	var apiKey string
	flag.IntVar(&port, "port", 8080, "API server port")
	flag.StringVar(&apiKey, "api-key", "", "API key for authentication (optional, can be set via registration)")
	flag.Parse()

	logger.InitLogger(logging.INFO)

	// Initialize node configuration system
	// Try to find config directory (same as XRAY config)
	configDirs := []string{"bin", "config", ".", "/app/bin", "/app/config"}
	var configDir string
	for _, dir := range configDirs {
		if _, err := os.Stat(dir); err == nil {
			configDir = dir
			break
		}
	}
	if configDir == "" {
		configDir = "." // Fallback
	}

	if err := nodeConfig.InitConfig(configDir); err != nil {
		log.Fatalf("Failed to initialize node config: %v", err)
	}

	// Get API key from (in order of priority):
	// 1. Command line flag
	// 2. Environment variable (for backward compatibility)
	// 3. Saved config file (from registration)
	if apiKey == "" {
		apiKey = os.Getenv("NODE_API_KEY")
	}
	if apiKey == "" {
		// Try to load from saved config
		savedConfig := nodeConfig.GetConfig()
		if savedConfig.ApiKey != "" {
			apiKey = savedConfig.ApiKey
			log.Printf("Using API key from saved configuration")
		}
	}

	// If still no API key, node can start but will need registration
	if apiKey == "" {
		log.Printf("WARNING: No API key found. Node will need to be registered via /api/v1/register endpoint")
		log.Printf("You can set NODE_API_KEY environment variable or use -api-key flag for immediate use")
		// Use a temporary key that will be replaced during registration
		apiKey = "temp-unregistered"
	}

	// Initialize log pusher if panel URL is configured
	// Get node address from saved config or environment variable
	savedConfig := nodeConfig.GetConfig()
	nodeAddress := savedConfig.NodeAddress
	if nodeAddress == "" {
		nodeAddress = os.Getenv("NODE_ADDRESS")
	}
	if nodeAddress == "" {
		// Default to localhost with the port (panel will match by port if address doesn't match exactly)
		nodeAddress = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	
	// Get panel URL from saved config or environment variable
	panelURL := savedConfig.PanelURL
	if panelURL == "" {
		panelURL = os.Getenv("PANEL_URL")
	}
	
	nodeLogs.InitLogPusher(nodeAddress)
	if panelURL != "" {
		nodeLogs.SetPanelURL(panelURL)
	}
	// Connect log pusher to logger
	logger.SetLogPusher(nodeLogs.PushLog)

	xrayManager := xray.NewManager()
	server := api.NewServer(port, apiKey, xrayManager)

	log.Printf("Starting 3x-ui Node Service on port %d", port)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	xrayManager.Stop()
	server.Stop()
	log.Println("Shutdown complete")
}
