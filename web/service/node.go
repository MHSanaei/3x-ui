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
	return nodes, err
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
			Uptime uint64 `json:"uptime"`
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
	patch.UptimeSecs = o.Uptime
	return patch, nil
}

type ProbeResultUI struct {
	Status      string  `json:"status"`
	LatencyMs   int     `json:"latencyMs"`
	XrayVersion string  `json:"xrayVersion"`
	CpuPct      float64 `json:"cpuPct"`
	MemPct      float64 `json:"memPct"`
	UptimeSecs  uint64  `json:"uptimeSecs"`
	Error       string  `json:"error"`
}

func (p HeartbeatPatch) ToUI(ok bool) ProbeResultUI {
	r := ProbeResultUI{
		LatencyMs:   p.LatencyMs,
		XrayVersion: p.XrayVersion,
		CpuPct:      p.CpuPct,
		MemPct:      p.MemPct,
		UptimeSecs:  p.UptimeSecs,
		Error:       p.LastError,
	}
	if ok {
		r.Status = "online"
	} else {
		r.Status = "offline"
	}
	return r
}
