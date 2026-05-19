package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/util/common"
	"github.com/mhsanaei/3x-ui/v3/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/web/runtime"
)

type HeartbeatPatch struct {
	Status        string
	LastHeartbeat int64
	LatencyMs     int
	XrayVersion   string
	PanelVersion  string
	CpuPct        float64
	MemPct        float64
	UptimeSecs    uint64
	LastError     string
}

type NodeService struct{}

var nodeHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
		DialContext:         netsafe.SSRFGuardedDialContext,
	},
}

func (s *NodeService) GetAll() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	err := db.Model(model.Node{}).Order("id asc").Find(&nodes).Error
	if err != nil || len(nodes) == 0 {
		return nodes, err
	}

	type inboundRow struct {
		Id     int
		NodeID int `gorm:"column:node_id"`
	}
	var inboundRows []inboundRow
	if err := db.Table("inbounds").
		Select("id, node_id").
		Where("node_id IS NOT NULL").
		Scan(&inboundRows).Error; err != nil {
		return nodes, nil
	}
	if len(inboundRows) == 0 {
		return nodes, nil
	}
	inboundsByNode := make(map[int][]int, len(nodes))
	nodeByInbound := make(map[int]int, len(inboundRows))
	for _, row := range inboundRows {
		inboundsByNode[row.NodeID] = append(inboundsByNode[row.NodeID], row.Id)
		nodeByInbound[row.Id] = row.NodeID
	}

	type clientCountRow struct {
		NodeID int `gorm:"column:node_id"`
		Count  int `gorm:"column:count"`
	}
	var clientCounts []clientCountRow
	if err := db.Raw(`
		SELECT inbounds.node_id AS node_id, COUNT(DISTINCT client_inbounds.client_id) AS count
		FROM inbounds
		JOIN client_inbounds ON client_inbounds.inbound_id = inbounds.id
		WHERE inbounds.node_id IS NOT NULL
		GROUP BY inbounds.node_id
	`).Scan(&clientCounts).Error; err == nil {
		for _, row := range clientCounts {
			for _, n := range nodes {
				if n.Id == row.NodeID {
					n.ClientCount = row.Count
					break
				}
			}
		}
	}

	now := time.Now().UnixMilli()
	type trafficRow struct {
		InboundID  int `gorm:"column:inbound_id"`
		Email      string
		Enable     bool
		Total      int64
		Up         int64
		Down       int64
		ExpiryTime int64 `gorm:"column:expiry_time"`
	}
	var trafficRows []trafficRow
	inboundIDs := make([]int, 0, len(nodeByInbound))
	for id := range nodeByInbound {
		inboundIDs = append(inboundIDs, id)
	}
	if err := db.Table("client_traffics").
		Select("inbound_id, email, enable, total, up, down, expiry_time").
		Where("inbound_id IN ?", inboundIDs).
		Scan(&trafficRows).Error; err == nil {
		online := make(map[string]struct{})
		for _, email := range s.onlineEmails() {
			online[email] = struct{}{}
		}
		depletedByNode := make(map[int]int)
		onlineByNode := make(map[int]int)
		for _, row := range trafficRows {
			nodeID, ok := nodeByInbound[row.InboundID]
			if !ok {
				continue
			}
			expired := row.ExpiryTime > 0 && row.ExpiryTime <= now
			exhausted := row.Total > 0 && row.Up+row.Down >= row.Total
			if expired || exhausted || !row.Enable {
				depletedByNode[nodeID]++
			}
			if _, ok := online[row.Email]; ok {
				onlineByNode[nodeID]++
			}
		}
		for _, n := range nodes {
			n.InboundCount = len(inboundsByNode[n.Id])
			n.DepletedCount = depletedByNode[n.Id]
			n.OnlineCount = onlineByNode[n.Id]
		}
	}

	return nodes, nil
}

func (s *NodeService) onlineEmails() []string {
	svc := InboundService{}
	return svc.GetOnlineClients()
}

func (s *NodeService) GetById(id int) (*model.Node, error) {
	db := database.GetDB()
	n := &model.Node{}
	if err := db.Model(model.Node{}).Where("id = ?", id).First(n).Error; err != nil {
		return nil, err
	}
	return n, nil
}

func normalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}
	return p
}

func (s *NodeService) normalize(n *model.Node) error {
	n.Name = strings.TrimSpace(n.Name)
	n.ApiToken = strings.TrimSpace(n.ApiToken)
	if n.Name == "" {
		return common.NewError("node name is required")
	}
	addr, err := netsafe.NormalizeHost(n.Address)
	if err != nil {
		return common.NewError(err.Error())
	}
	n.Address = addr
	if n.Port <= 0 || n.Port > 65535 {
		return common.NewError("node port must be 1-65535")
	}
	if n.Scheme != "http" && n.Scheme != "https" {
		n.Scheme = "https"
	}
	n.BasePath = normalizeBasePath(n.BasePath)
	return nil
}

func (s *NodeService) Create(n *model.Node) error {
	if err := s.normalize(n); err != nil {
		return err
	}
	db := database.GetDB()
	return db.Create(n).Error
}

func (s *NodeService) Update(id int, in *model.Node) error {
	if err := s.normalize(in); err != nil {
		return err
	}
	db := database.GetDB()
	existing := &model.Node{}
	if err := db.Where("id = ?", id).First(existing).Error; err != nil {
		return err
	}
	updates := map[string]any{
		"name":                  in.Name,
		"remark":                in.Remark,
		"scheme":                in.Scheme,
		"address":               in.Address,
		"port":                  in.Port,
		"base_path":             in.BasePath,
		"api_token":             in.ApiToken,
		"enable":                in.Enable,
		"allow_private_address": in.AllowPrivateAddress,
	}
	if err := db.Model(model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	if mgr := runtime.GetManager(); mgr != nil {
		mgr.InvalidateNode(id)
	}
	return nil
}

func (s *NodeService) Delete(id int) error {
	db := database.GetDB()
	if err := db.Where("id = ?", id).Delete(model.Node{}).Error; err != nil {
		return err
	}
	if mgr := runtime.GetManager(); mgr != nil {
		mgr.InvalidateNode(id)
	}
	nodeMetrics.drop(nodeMetricKey(id, "cpu"))
	nodeMetrics.drop(nodeMetricKey(id, "mem"))
	return nil
}

func (s *NodeService) SetEnable(id int, enable bool) error {
	db := database.GetDB()
	return db.Model(model.Node{}).Where("id = ?", id).Update("enable", enable).Error
}

func (s *NodeService) UpdateHeartbeat(id int, p HeartbeatPatch) error {
	db := database.GetDB()
	updates := map[string]any{
		"status":         p.Status,
		"last_heartbeat": p.LastHeartbeat,
		"latency_ms":     p.LatencyMs,
		"xray_version":   p.XrayVersion,
		"panel_version":  p.PanelVersion,
		"cpu_pct":        p.CpuPct,
		"mem_pct":        p.MemPct,
		"uptime_secs":    p.UptimeSecs,
		"last_error":     p.LastError,
	}
	if err := db.Model(model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	if p.Status == "online" {
		now := time.Unix(p.LastHeartbeat, 0)
		nodeMetrics.append(nodeMetricKey(id, "cpu"), now, p.CpuPct)
		nodeMetrics.append(nodeMetricKey(id, "mem"), now, p.MemPct)
	}
	return nil
}

func nodeMetricKey(id int, metric string) string {
	return "node:" + strconv.Itoa(id) + ":" + metric
}

func (s *NodeService) AggregateNodeMetric(id int, metric string, bucketSeconds int, maxPoints int) []map[string]any {
	return nodeMetrics.aggregate(nodeMetricKey(id, metric), bucketSeconds, maxPoints)
}

func (s *NodeService) Probe(ctx context.Context, n *model.Node) (HeartbeatPatch, error) {
	patch := HeartbeatPatch{LastHeartbeat: time.Now().Unix()}

	addr, err := netsafe.NormalizeHost(n.Address)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	scheme := n.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if n.Port <= 0 || n.Port > 65535 {
		patch.LastError = "node port must be 1-65535"
		return patch, errors.New(patch.LastError)
	}
	probeURL := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(addr, strconv.Itoa(n.Port)),
		Path:   normalizeBasePath(n.BasePath) + "panel/api/server/status",
	}

	req, err := http.NewRequestWithContext(
		netsafe.ContextWithAllowPrivate(ctx, n.AllowPrivateAddress),
		http.MethodGet, probeURL.String(), nil)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	if n.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.ApiToken)
	}
	req.Header.Set("Accept", "application/json")

	start := time.Now()
	resp, err := nodeHTTPClient.Do(req)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	defer resp.Body.Close()
	patch.LatencyMs = int(time.Since(start) / time.Millisecond)

	if resp.StatusCode != http.StatusOK {
		patch.LastError = fmt.Sprintf("HTTP %d from remote panel", resp.StatusCode)
		return patch, errors.New(patch.LastError)
	}

	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     *struct {
			CpuPct float64 `json:"cpu"`
			Mem    struct {
				Current uint64 `json:"current"`
				Total   uint64 `json:"total"`
			} `json:"mem"`
			Xray struct {
				Version string `json:"version"`
			} `json:"xray"`
			PanelVersion string `json:"panelVersion"`
			Uptime       uint64 `json:"uptime"`
		} `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		patch.LastError = "decode response: " + err.Error()
		return patch, err
	}
	if !envelope.Success || envelope.Obj == nil {
		patch.LastError = "remote returned success=false: " + envelope.Msg
		return patch, errors.New(patch.LastError)
	}
	o := envelope.Obj
	patch.CpuPct = o.CpuPct
	if o.Mem.Total > 0 {
		patch.MemPct = float64(o.Mem.Current) * 100.0 / float64(o.Mem.Total)
	}
	patch.XrayVersion = o.Xray.Version
	patch.PanelVersion = o.PanelVersion
	patch.UptimeSecs = o.Uptime
	return patch, nil
}

type ProbeResultUI struct {
	Status       string  `json:"status"`
	LatencyMs    int     `json:"latencyMs"`
	XrayVersion  string  `json:"xrayVersion"`
	PanelVersion string  `json:"panelVersion"`
	CpuPct       float64 `json:"cpuPct"`
	MemPct       float64 `json:"memPct"`
	UptimeSecs   uint64  `json:"uptimeSecs"`
	Error        string  `json:"error"`
}

func (p HeartbeatPatch) ToUI(ok bool) ProbeResultUI {
	r := ProbeResultUI{
		LatencyMs:    p.LatencyMs,
		XrayVersion:  p.XrayVersion,
		PanelVersion: p.PanelVersion,
		CpuPct:       p.CpuPct,
		MemPct:       p.MemPct,
		UptimeSecs:   p.UptimeSecs,
		Error:        p.LastError,
	}
	if ok {
		r.Status = "online"
	} else {
		r.Status = "offline"
	}
	return r
}
