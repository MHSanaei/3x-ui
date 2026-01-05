// Package main is the entry point for the 3x-ui node service (worker).
// This service runs XRAY Core and provides a REST API for the master panel to manage it.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/node/api"
	"github.com/mhsanaei/3x-ui/v2/node/xray"
	"github.com/op/go-logging"
)

func main() {
	var port int
	var apiKey string
	flag.IntVar(&port, "port", 8080, "API server port")
	flag.StringVar(&apiKey, "api-key", "", "API key for authentication (required)")
	flag.Parse()

	// Check environment variable if flag is not provided
	if apiKey == "" {
		apiKey = os.Getenv("NODE_API_KEY")
	}

	if apiKey == "" {
		log.Fatal("API key is required. Set NODE_API_KEY environment variable or use -api-key flag")
	}

	logger.InitLogger(logging.INFO)

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
