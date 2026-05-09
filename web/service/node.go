package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/web/runtime"
)

// HeartbeatPatch is the slice of fields a single Probe() result writes
// back to a Node row. We pass it as a struct (not a *model.Node) so the
// heartbeat path can't accidentally clobber configuration columns the
// user just edited.
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

// NodeService manages remote 3x-ui nodes registered with this panel.
// It owns CRUD for the Node model and the HTTP probe used by both the
// heartbeat job and the on-demand "test connection" UI action.
type NodeService struct{}

// httpClient is shared so repeated probes reuse TCP/TLS connections.
// Timeout is per-request, set on each Do() via context.
var nodeHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
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

// normalize fills in defaults and trims accidental whitespace before save.
// Pulled out so Create and Update share the same rules.
func (s *NodeService) normalize(n *model.Node) error {
	n.Name = strings.TrimSpace(n.Name)
	n.Address = strings.TrimSpace(n.Address)
	n.ApiToken = strings.TrimSpace(n.ApiToken)
	if n.Name == "" {
		return common.NewError("node name is required")
	}
	if n.Address == "" {
		return common.NewError("node address is required")
	}
	if n.Port <= 0 || n.Port > 65535 {
		return common.NewError("node port must be 1-65535")
	}
	if n.Scheme != "http" && n.Scheme != "https" {
		n.Scheme = "https"
	}
	if n.BasePath == "" {
		n.BasePath = "/"
	}
	if !strings.HasPrefix(n.BasePath, "/") {
		n.BasePath = "/" + n.BasePath
	}
	if !strings.HasSuffix(n.BasePath, "/") {
		n.BasePath = n.BasePath + "/"
	}
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
	// Only persist user-controlled columns. Heartbeat fields stay where
	// the heartbeat job last wrote them so a no-op edit doesn't blank
	// the dashboard out for ten seconds.
	updates := map[string]any{
		"name":      in.Name,
		"remark":    in.Remark,
		"scheme":    in.Scheme,
		"address":   in.Address,
		"port":      in.Port,
		"base_path": in.BasePath,
		"api_token": in.ApiToken,
		"enable":    in.Enable,
	}
	if err := db.Model(model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	// Drop any cached Remote so the next inbound op picks up the fresh
	// address/token. Cheap to do unconditionally — the next miss rebuilds.
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
	// Drop in-memory series so a freshly created node with the same id
	// doesn't inherit stale points (sqlite reuses ids freely).
	nodeMetrics.drop(nodeMetricKey(id, "cpu"))
	nodeMetrics.drop(nodeMetricKey(id, "mem"))
	return nil
}

func (s *NodeService) SetEnable(id int, enable bool) error {
	db := database.GetDB()
	return db.Model(model.Node{}).Where("id = ?", id).Update("enable", enable).Error
}

// UpdateHeartbeat persists the slice of fields written by a probe. We
// don't touch updated_at via gorm autoUpdateTime here — that field is
// reserved for user-driven config edits.
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
	// Only record online ticks. Offline probes carry zeroed cpu/mem and
	// would draw a misleading dip on the chart; the gap on the x-axis is
	// the truthful representation of "we couldn't reach the node".
	if p.Status == "online" {
		now := time.Unix(p.LastHeartbeat, 0)
		nodeMetrics.append(nodeMetricKey(id, "cpu"), now, p.CpuPct)
		nodeMetrics.append(nodeMetricKey(id, "mem"), now, p.MemPct)
	}
	return nil
}

// nodeMetricKey is the namespacing used inside the singleton ring buffer
// so per-node metrics don't collide with each other or with the system
// metrics in the sibling singleton.
func nodeMetricKey(id int, metric string) string {
	return "node:" + strconv.Itoa(id) + ":" + metric
}

// AggregateNodeMetric returns up to maxPoints averaged buckets for one
// node's metric (currently "cpu" or "mem"). Output shape matches
// AggregateSystemMetric: {"t": unixSec, "v": value}.
func (s *NodeService) AggregateNodeMetric(id int, metric string, bucketSeconds int, maxPoints int) []map[string]any {
	return nodeMetrics.aggregate(nodeMetricKey(id, metric), bucketSeconds, maxPoints)
}

// Probe issues a single GET to the node's /panel/api/server/status and
// returns a HeartbeatPatch. On error the patch is zero-valued except
// for LastError; the caller is responsible for setting Status="offline".
//
// The remote endpoint requires authentication: we send the per-node
// ApiToken as a Bearer token, which the remote APIController.checkAPIAuth
// validates. Calls without a token would just get a 404, which masks
// the existence of the API entirely.
func (s *NodeService) Probe(ctx context.Context, n *model.Node) (HeartbeatPatch, error) {
	patch := HeartbeatPatch{LastHeartbeat: time.Now().Unix()}
	url := fmt.Sprintf("%s://%s:%d%spanel/api/server/status",
		n.Scheme, n.Address, n.Port, n.BasePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

	// The remote wraps Status in entity.Msg. We decode into a typed
	// envelope rather than map[string]any so a schema change on the
	// remote shows up as a Go error instead of a silent zero-fill.
	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     *struct {
			Cpu uint64 `json:"-"`
			// Status fields we care about. Decode CPU/Mem nested
			// structs minimally — anything else gets discarded.
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

// EnvelopeForUI is the shape a frontend test-connection action expects.
// Pulling it out keeps the controller dumb.
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

