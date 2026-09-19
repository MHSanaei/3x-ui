package singbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"sort"
)

const clashAPIAddress = "http://127.0.0.1:10090"

type ClashConnection struct {
	ID string `json:"id"`
	Upload int64 `json:"upload"`
	Download int64 `json:"download"`
	Start time.Time `json:"start"`
	Chains []string `json:"chains"`
	Metadata ClashMetadata `json:"metadata"`
}

type ClashMetadata struct {
	Network string `json:"network"`
	Type string `json:"type"`
	SourceIP string `json:"sourceIP"`
	SourcePort string `json:"sourcePort"`
	DestinationIP string `json:"destinationIP"`
	DestinationPort string `json:"destinationPort"`
	Host string `json:"host"`
	DNSMode string `json:"dnsMode"`
	ProcessPath string `json:"processPath"`
	User string `json:"user"`
}

type clashConnectionsResponse struct {
	Connections []ClashConnection `json:"connections"`
	UploadTotal int64 `json:"uploadTotal"`
	DownloadTotal int64 `json:"downloadTotal"`
}

type ClashStatsClient struct {
	client *http.Client
}

func NewClashStatsClient() *ClashStatsClient {
	return &ClashStatsClient{client: &http.Client{Timeout: 2 * time.Second}}
}

func (c *ClashStatsClient) Connections(ctx context.Context) ([]ClashConnection, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clashAPIAddress+"/connections", nil)
	if err != nil { return nil, err }
	resp, err := c.client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sing-box Clash API returned HTTP %d", resp.StatusCode)
	}
	var payload clashConnectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil { return nil, err }
	return payload.Connections, nil
}

func (c *ClashStatsClient) UserEmails(ctx context.Context) ([]string, error) {
	connections, err := c.Connections(ctx)
	if err != nil {
		return nil, err
	}
	users := make(map[string]struct{})
	for _, connection := range connections {
		if connection.Metadata.User != "" {
			users[connection.Metadata.User] = struct{}{}
		}
	}
	result := make([]string, 0, len(users))
	for user := range users {
		result = append(result, user)
	}
	sort.Strings(result)
	return result, nil
}

func (c *ClashStatsClient) OnlineIPSet(ctx context.Context) (map[string]map[string]struct{}, int, error) {
	connections, err := c.Connections(ctx)
	if err != nil { return nil, 0, err }
	byInbound := make(map[string]map[string]struct{})
	for _, connection := range connections {
		ip := connection.Metadata.SourceIP
		if ip == "" { continue }
		inbound := connection.Metadata.Type
		if inbound == "" { inbound = "unknown" }
		if byInbound[inbound] == nil { byInbound[inbound] = make(map[string]struct{}) }
		byInbound[inbound][ip] = struct{}{}
	}
	return byInbound, len(connections), nil
}
