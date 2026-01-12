// Package job provides scheduled background jobs for the 3x-ui panel.
package job

import (
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// logEntry represents a log entry with timestamp for sorting
type logEntry struct {
	timestamp time.Time
	nodeName  string
	level     string
	message   string
}

// CollectNodeLogsJob periodically collects XRAY logs from nodes and adds them to the panel log buffer.
type CollectNodeLogsJob struct {
	nodeService service.NodeService
	// Track last collected log hash for each node and log type to avoid duplicates
	lastLogHashes map[string]string // key: "nodeId:logType" (e.g., "1:service", "1:access")
	mu            sync.RWMutex
}

// NewCollectNodeLogsJob creates a new job for collecting node logs.
func NewCollectNodeLogsJob() *CollectNodeLogsJob {
	return &CollectNodeLogsJob{
		nodeService:  service.NodeService{},
		lastLogHashes: make(map[string]string),
	}
}

// parseLogLine parses a log line from node and extracts timestamp, level, and message.
// Format: "timestamp level - message" or "timestamp level - message" for access logs.
// Returns timestamp, level, message, and success flag.
func (j *CollectNodeLogsJob) parseLogLine(logLine string) (time.Time, string, string, bool) {
	// Try to parse format: "2006/01/02 15:04:05 level - message" or "2006/01/02 15:04:05.999999 level - message"
	if idx := strings.Index(logLine, " - "); idx != -1 {
		parts := strings.SplitN(logLine, " - ", 2)
		if len(parts) == 2 {
			// parts[0] = "timestamp level", parts[1] = "message"
			levelPart := strings.TrimSpace(parts[0])
			levelFields := strings.Fields(levelPart)
			
			if len(levelFields) >= 3 {
				// Format: "2006/01/02 15:04:05 level" or "2006/01/02 15:04:05.999999 level"
				timestampStr := levelFields[0] + " " + levelFields[1]
				level := strings.ToUpper(levelFields[2])
				message := parts[1]
				
				// Try parsing with microseconds first
				timestamp, err := time.ParseInLocation("2006/01/02 15:04:05.999999", timestampStr, time.Local)
				if err != nil {
					// Fallback to format without microseconds
					timestamp, err = time.ParseInLocation("2006/01/02 15:04:05", timestampStr, time.Local)
				}
				
				if err == nil {
					return timestamp, level, message, true
				}
			} else if len(levelFields) >= 2 {
				// Try to parse as "timestamp level" where timestamp might be in different format
				level := strings.ToUpper(levelFields[len(levelFields)-1])
				message := parts[1]
				// Try to extract timestamp from first fields
				if len(levelFields) >= 2 {
					timestampStr := strings.Join(levelFields[:len(levelFields)-1], " ")
					timestamp, err := time.ParseInLocation("2006/01/02 15:04:05.999999", timestampStr, time.Local)
					if err != nil {
						timestamp, err = time.ParseInLocation("2006/01/02 15:04:05", timestampStr, time.Local)
					}
					if err == nil {
						return timestamp, level, message, true
					}
				}
				// If timestamp parsing fails, use current time
				return time.Now(), level, message, true
			}
		}
	}
	
	// If parsing fails, return current time and treat as INFO level
	return time.Now(), "INFO", logLine, false
}

// processLogs processes logs from a node and returns new log entries with timestamps.
// logType can be "service" or "access" to track them separately.
func (j *CollectNodeLogsJob) processLogs(node *model.Node, rawLogs []string, logType string) []logEntry {
	if len(rawLogs) == 0 {
		return nil
	}

	// Get last collected log hash for this node and log type
	hashKey := fmt.Sprintf("%d:%s", node.Id, logType)
	j.mu.RLock()
	lastHash := j.lastLogHashes[hashKey]
	j.mu.RUnlock()

	// Process logs from newest to oldest to find where we left off
	var newLogLines []string
	var mostRecentHash string
	foundLastHash := lastHash == "" // If no last hash, all logs are new

	// Iterate from newest (end) to oldest (start)
	for i := len(rawLogs) - 1; i >= 0; i-- {
		logLine := rawLogs[i]
		if logLine == "" {
			continue
		}

		// Skip API calls for access logs
		if logType == "access" && strings.Contains(logLine, "api -> api") {
			continue
		}

		// Calculate hash for this log line
		logHash := fmt.Sprintf("%x", md5.Sum([]byte(logLine)))

		// Store the most recent hash (first valid log we encounter)
		if mostRecentHash == "" {
			mostRecentHash = logHash
		}

		// If we haven't found the last collected log yet, check if this is it
		if !foundLastHash {
			if logHash == lastHash {
				foundLastHash = true
			}
			continue
		}

		// This is a new log (after the last collected one)
		newLogLines = append(newLogLines, logLine)
	}

	// If we didn't find the last hash, all logs in this batch are new
	if !foundLastHash {
		// Add all valid logs as new
		for i := len(rawLogs) - 1; i >= 0; i-- {
			logLine := rawLogs[i]
			if logLine != "" {
				if logType == "access" && strings.Contains(logLine, "api -> api") {
					continue
				}
				newLogLines = append(newLogLines, logLine)
			}
		}
	}

	// Parse logs and create entries with timestamps
	var entries []logEntry
	for _, logLine := range newLogLines {
		timestamp, level, message, _ := j.parseLogLine(logLine)
		entries = append(entries, logEntry{
			timestamp: timestamp,
			nodeName:  node.Name,
			level:     level,
			message:   message,
		})
	}

	// Update last hash to the most recent log we processed (newest log from batch)
	if mostRecentHash != "" {
		j.mu.Lock()
		j.lastLogHashes[hashKey] = mostRecentHash
		j.mu.Unlock()
	}

	return entries
}

// Run executes the job to collect logs from all nodes and add them to the panel log buffer.
func (j *CollectNodeLogsJob) Run() {
	// Check if multi-node mode is enabled
	settingService := service.SettingService{}
	multiMode, err := settingService.GetMultiNodeMode()
	if err != nil || !multiMode {
		return // Skip if multi-node mode is not enabled
	}

	nodes, err := j.nodeService.GetAllNodes()
	if err != nil {
		logger.Debugf("Failed to get nodes for log collection: %v", err)
		return
	}

	if len(nodes) == 0 {
		return // No nodes to collect logs from
	}

	// Collect all logs from all nodes first, then sort and add to buffer
	var allEntries []logEntry
	var wg sync.WaitGroup
	var entriesMu sync.Mutex

	// Collect logs from each node concurrently
	// Only collect from nodes that have assigned inbounds (active nodes)
	for _, node := range nodes {
		n := node // Capture loop variable
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// Check if node has assigned inbounds (only collect from active nodes)
			inbounds, err := j.nodeService.GetInboundsForNode(n.Id)
			if err != nil || len(inbounds) == 0 {
				return // Skip nodes without assigned inbounds
			}

			var nodeEntries []logEntry

			// Collect service logs (node service and XRAY core logs)
			serviceLogs, err := j.nodeService.GetNodeServiceLogs(n, 100, "debug")
			if err != nil {
				// Don't log errors for offline nodes
				if !strings.Contains(err.Error(), "status code") {
					logger.Debugf("[Node: %s] Failed to collect service logs: %v", n.Name, err)
				}
			} else {
				// Process service logs
				nodeEntries = append(nodeEntries, j.processLogs(n, serviceLogs, "service")...)
			}

			// Get XRAY access logs (traffic logs)
			rawLogs, err := j.nodeService.GetNodeLogs(n, 100, "")
			if err != nil {
				// Don't log errors for offline nodes or nodes without logs configured
				if !strings.Contains(err.Error(), "XRAY is not running") &&
					!strings.Contains(err.Error(), "status code") &&
					!strings.Contains(err.Error(), "access log path") {
					logger.Debugf("[Node: %s] Failed to collect access logs: %v", n.Name, err)
				}
			} else {
				// Process access logs
				nodeEntries = append(nodeEntries, j.processLogs(n, rawLogs, "access")...)
			}

			// Add node entries to global list
			if len(nodeEntries) > 0 {
				entriesMu.Lock()
				allEntries = append(allEntries, nodeEntries...)
				entriesMu.Unlock()
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Sort all entries by timestamp (oldest first)
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].timestamp.Before(allEntries[j].timestamp)
	})

	// Add sorted logs to panel buffer
	for _, entry := range allEntries {
		formattedMessage := fmt.Sprintf("[Node: %s] %s", entry.nodeName, entry.message)
		switch entry.level {
		case "DEBUG":
			logger.Debugf("%s", formattedMessage)
		case "WARNING":
			logger.Warningf("%s", formattedMessage)
		case "ERROR":
			logger.Errorf("%s", formattedMessage)
		case "NOTICE":
			logger.Noticef("%s", formattedMessage)
		default:
			logger.Infof("%s", formattedMessage)
		}
	}

	if len(allEntries) > 0 {
		logger.Debugf("Collected and sorted %d new log entries from %d nodes", len(allEntries), len(nodes))
	}
}
