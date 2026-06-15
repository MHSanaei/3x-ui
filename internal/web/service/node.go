package service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type HeartbeatPatch struct {
	Status        string
	LastHeartbeat int64
	LatencyMs     int
	XrayVersion   string
	PanelVersion  string
	Guid          string
	CpuPct        float64
	MemPct        float64
	UptimeSecs    uint64
	LastError     string
	// XrayState and XrayError come from the remote /panel/api/server/status when the
	// panel API is reachable. They allow distinguishing panel connectivity from
	// Xray core health on the node.
	XrayState string
	XrayError string
}

type NodeService struct{}

// FetchCertFingerprint connects to the node over HTTPS without verifying the
// certificate and returns the leaf certificate's SHA-256 as base64, so the UI
// can offer a "fetch and pin current certificate" action.
func (s *NodeService) FetchCertFingerprint(ctx context.Context, n *model.Node) (string, error) {
	addr, err := netsafe.NormalizeHost(n.Address)
	if err != nil {
		return "", err
	}
	scheme := n.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if scheme != "https" {
		return "", common.NewError("certificate pinning is only available for https nodes")
	}
	if n.Port <= 0 || n.Port > 65535 {
		return "", common.NewError("node port must be 1-65535")
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
		return "", err
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:     netsafe.SSRFGuardedDialContext,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // lgtm[go/disabled-certificate-check]
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return "", common.NewError("node did not present a TLS certificate")
	}
	sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
	return base64.StdEncoding.EncodeToString(sum[:]), nil
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
	// Chunk the IN clause to avoid "too many SQL variables" on SQLite
	// when there are many node-owned inbounds (common with many nodes).
	// sqliteMaxVars is defined in this package (inbound.go).
	for _, batch := range chunkInts(inboundIDs, sqliteMaxVars) {
		var page []trafficRow
		if err := db.Table("client_traffics").
			Select("inbound_id, email, enable, total, up, down, expiry_time").
			Where("inbound_id IN ?", batch).
			Scan(&page).Error; err == nil {
			trafficRows = append(trafficRows, page...)
		}
	}
	depletedByNode := make(map[int]int)
	if len(trafficRows) > 0 {
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
		}
	}
	onlineByGuid := s.onlineEmailsByGuid()
	for _, n := range nodes {
		n.InboundCount = len(inboundsByNode[n.Id])
		n.DepletedCount = depletedByNode[n.Id]
		// Online is attributed to the node that physically hosts the client
		// (by GUID): a client on a sub-node counts under the sub-node, not
		// the intermediate node it syncs through (#4983).
		n.OnlineCount = len(onlineByGuid[effectiveNodeGuid(n)])
	}

	return nodes, nil
}

func (s *NodeService) onlineEmailsByGuid() map[string]map[string]struct{} {
	svc := InboundService{}
	byGuid := svc.GetOnlineClientsByGuid()
	out := make(map[string]map[string]struct{}, len(byGuid))
	for guid, emails := range byGuid {
		set := make(map[string]struct{}, len(emails))
		for _, email := range emails {
			set[email] = struct{}{}
		}
		out[guid] = set
	}
	return out
}

// effectiveNodeGuid is a node's stable online-attribution key: its reported
// panelGuid, or a master-local synthetic id when the node is an old build that
// hasn't reported one yet (#4983).
func effectiveNodeGuid(n *model.Node) string {
	if n.Guid != "" {
		return n.Guid
	}
	return synthNodeGuid(n.Id)
}

func (s *NodeService) GetById(id int) (*model.Node, error) {
	db := database.GetDB()
	n := &model.Node{}
	if err := db.Model(model.Node{}).Where("id = ?", id).First(n).Error; err != nil {
		return nil, err
	}
	return n, nil
}

// NodeExists reports whether a node with the given id exists on this panel.
// Used to drop stale, cross-panel node references on inbound import. A Count
// query distinguishes "no such node" (count 0, no error) from a real DB error.
func (s *NodeService) NodeExists(id int) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	var count int64
	if err := database.GetDB().Model(model.Node{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
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
	if n.TlsVerifyMode != "skip" && n.TlsVerifyMode != "pin" {
		n.TlsVerifyMode = "verify"
	}
	n.PinnedCertSha256 = strings.TrimSpace(n.PinnedCertSha256)
	if n.InboundSyncMode != "selected" {
		n.InboundSyncMode = "all"
		n.InboundTags = nil
	} else {
		seen := make(map[string]struct{}, len(n.InboundTags))
		tags := make([]string, 0, len(n.InboundTags))
		for _, tag := range n.InboundTags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
		n.InboundTags = tags
	}
	if n.TlsVerifyMode == "pin" {
		if _, err := runtime.DecodeCertPin(n.PinnedCertSha256); err != nil {
			return common.NewError(err.Error())
		}
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
	inboundTagsJSON, err := json.Marshal(in.InboundTags)
	if err != nil {
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
		"tls_verify_mode":       in.TlsVerifyMode,
		"pinned_cert_sha256":    in.PinnedCertSha256,
		"inbound_sync_mode":     in.InboundSyncMode,
		"inbound_tags":          string(inboundTagsJSON),
		"outbound_tag":          in.OutboundTag,
	}
	if err := db.Model(model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	if mgr := runtime.GetManager(); mgr != nil {
		mgr.InvalidateNode(id)
	}
	return nil
}

func (s *NodeService) GetRemoteInboundOptions(ctx context.Context, n *model.Node) ([]runtime.RemoteInboundOption, error) {
	if err := s.normalize(n); err != nil {
		return nil, err
	}
	if n.OutboundTag == "" {
		return runtime.NewRemote(n, nil).ListInboundOptions(ctx)
	}
	// Mirror ProbeWithOutbound: a node being added/edited has no persistent
	// egress bridge yet, so route the list call through a temporary one or the
	// remote panel stays unreachable and the request times out.
	var options []runtime.RemoteInboundOption
	var err error
	s.withOutboundBridge(n.Id, n.OutboundTag, func(proxyURL string) {
		options, err = runtime.NewRemote(n, staticEgressResolver(proxyURL)).ListInboundOptions(ctx)
	})
	return options, err
}

// staticEgressResolver hands a fixed proxy URL to runtime.NewRemote. An empty
// string yields a direct connection, so it doubles as the graceful fallback
// when a temporary bridge can't be built.
type staticEgressResolver string

func (r staticEgressResolver) NodeEgressProxyURL(int) string { return string(r) }

// EnsureInboundTagAllowed adds a panel-managed inbound's tag to the node's
// selection when the node syncs in "selected" mode. Without it, the next
// traffic sync would filter the tag out of the snapshot and the orphan sweep
// would silently delete the central row the panel just created or renamed.
// Tags are only ever added (never removed): on a rename the node may keep
// reporting the old tag until the remote update lands, and a leftover entry
// that matches nothing is harmless.
func (s *NodeService) EnsureInboundTagAllowed(nodeID int, tag string) error {
	tag = strings.TrimSpace(tag)
	if nodeID <= 0 || tag == "" {
		return nil
	}
	db := database.GetDB()
	node := &model.Node{}
	if err := db.Where("id = ?", nodeID).First(node).Error; err != nil {
		return err
	}
	if node.InboundSyncMode != "selected" {
		return nil
	}
	for _, t := range node.InboundTags {
		if t == tag {
			return nil
		}
	}
	buf, err := json.Marshal(append(node.InboundTags, tag))
	if err != nil {
		return err
	}
	return db.Model(model.Node{}).Where("id = ?", nodeID).
		Updates(map[string]any{"inbound_tags": string(buf)}).Error
}

func FilterNodeSnapshot(n *model.Node, snap *runtime.TrafficSnapshot) {
	if n == nil || snap == nil || n.InboundSyncMode != "selected" {
		return
	}
	allowed := make(map[string]struct{}, len(n.InboundTags))
	for _, tag := range n.InboundTags {
		allowed[tag] = struct{}{}
	}
	filtered := make([]*model.Inbound, 0, len(snap.Inbounds))
	for _, inbound := range snap.Inbounds {
		if inbound == nil {
			continue
		}
		if _, ok := allowed[inbound.Tag]; ok {
			filtered = append(filtered, inbound)
		}
	}
	snap.Inbounds = filtered
}

func (s *NodeService) Delete(id int) error {
	db := database.GetDB()
	// Capture the node's guid before deleting the row so we can drop its per-node
	// IP attribution (NodeClientIp is keyed by guid, not node id).
	var guid string
	var n model.Node
	if err := db.Select("guid").Where("id = ?", id).First(&n).Error; err == nil {
		guid = n.Guid
	}
	if err := db.Where("id = ?", id).Delete(model.Node{}).Error; err != nil {
		return err
	}
	if err := db.Where("node_id = ?", id).Delete(&model.NodeClientTraffic{}).Error; err != nil {
		return err
	}
	if guid != "" {
		if err := db.Where("node_guid = ?", guid).Delete(&model.NodeClientIp{}).Error; err != nil {
			return err
		}
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
	if err := db.Model(model.Node{}).Where("id = ?", id).Update("enable", enable).Error; err != nil {
		return err
	}
	if mgr := runtime.GetManager(); mgr != nil {
		mgr.InvalidateNode(id)
	}
	return nil
}

// GetWebCertFiles asks a node for its own web TLS certificate/key file paths,
// used by "Set Cert from Panel" so a node-assigned inbound gets paths that
// exist on the node rather than the central panel. See issue #4854.
func (s *NodeService) GetWebCertFiles(id int) (*runtime.WebCertFiles, error) {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nil, fmt.Errorf("node not found")
	}
	if !n.Enable {
		return nil, fmt.Errorf("node is disabled")
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return remote.GetWebCertFiles(ctx)
}

// NodeUpdateResult reports the outcome of triggering a panel self-update on one
// node so the UI can show per-node success/failure for a bulk request.
type NodeUpdateResult struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// UpdatePanels triggers the official self-updater on each given node. Only
// enabled, online nodes are eligible — an offline node can't be reached, so it
// is reported as skipped rather than silently dropped.
func (s *NodeService) UpdatePanels(ids []int) ([]NodeUpdateResult, error) {
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager unavailable")
	}
	results := make([]NodeUpdateResult, 0, len(ids))
	for _, id := range ids {
		n, err := s.GetById(id)
		if err != nil || n == nil {
			results = append(results, NodeUpdateResult{Id: id, OK: false, Error: "node not found"})
			continue
		}
		res := NodeUpdateResult{Id: id, Name: n.Name}
		switch {
		case !n.Enable:
			res.Error = "node is disabled"
		case n.Status != "online":
			res.Error = "node is offline"
		default:
			remote, remoteErr := mgr.RemoteFor(n)
			if remoteErr != nil {
				res.Error = remoteErr.Error()
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			updErr := remote.UpdatePanel(ctx)
			cancel()
			if updErr != nil {
				res.Error = updErr.Error()
			} else {
				res.OK = true
			}
		}
		results = append(results, res)
	}
	return results, nil
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
		"xray_state":     p.XrayState,
		"xray_error":     p.XrayError,
	}
	// Only learn the GUID; never clear a known one if an old-build node (or a
	// failed probe) reports none, so the stable identity survives blips.
	if p.Guid != "" {
		updates["guid"] = p.Guid
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

func (s *NodeService) MarkNodeDirty(id int) error {
	if id <= 0 {
		return nil
	}
	return database.GetDB().Model(model.Node{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"config_dirty":    true,
			"config_dirty_at": time.Now().UnixMilli(),
		}).Error
}

func (s *NodeService) ClearNodeDirty(id int, dirtyAt int64) error {
	if id <= 0 {
		return nil
	}
	return database.GetDB().Model(model.Node{}).
		Where("id = ? AND config_dirty_at = ?", id, dirtyAt).
		Update("config_dirty", false).Error
}

func (s *NodeService) NodeSyncState(id int) (enabled bool, status string, dirty bool, dirtyAt int64, err error) {
	if id <= 0 {
		return false, "", false, 0, errors.New("invalid node id")
	}
	var row model.Node
	err = database.GetDB().Model(model.Node{}).
		Select("enable", "status", "config_dirty", "config_dirty_at").
		Where("id = ?", id).
		First(&row).Error
	if err != nil {
		return false, "", false, 0, err
	}
	return row.Enable, row.Status, row.ConfigDirty, row.ConfigDirtyAt, nil
}

func (s *NodeService) IsNodePending(id int) bool {
	enabled, status, dirty, _, err := s.NodeSyncState(id)
	if err != nil {
		return false
	}
	return !enabled || status != "online" || dirty
}

func nodeMetricKey(id int, metric string) string {
	return "node:" + strconv.Itoa(id) + ":" + metric
}

func (s *NodeService) AggregateNodeMetric(id int, metric string, bucketSeconds int, maxPoints int) []map[string]any {
	return nodeMetrics.aggregate(nodeMetricKey(id, metric), bucketSeconds, maxPoints)
}

func (s *NodeService) Probe(ctx context.Context, n *model.Node) (HeartbeatPatch, error) {
	proxyURL := ""
	if n.OutboundTag != "" {
		if mgr := runtime.GetManager(); mgr != nil {
			proxyURL = mgr.NodeEgressProxyURL(n.Id)
		}
	}
	return s.probe(ctx, n, proxyURL)
}

func (s *NodeService) ProbeWithOutbound(ctx context.Context, n *model.Node, outboundTag string) (HeartbeatPatch, error) {
	if outboundTag == "" {
		return s.Probe(ctx, n)
	}
	var patch HeartbeatPatch
	var err error
	s.withOutboundBridge(n.Id, outboundTag, func(proxyURL string) {
		if proxyURL == "" {
			patch, err = s.Probe(ctx, n)
			return
		}
		patch, err = s.probe(ctx, n, proxyURL)
	})
	return patch, err
}

// withOutboundBridge stands up a temporary loopback SOCKS5 inbound in the
// running Xray, routes it through outboundTag, and runs fn with the bridge's
// proxy URL before tearing it down. It is used to reach a node through its
// connection outbound before the persistent egress bridge has been injected
// into the config (e.g. while the node is still being added or edited). When
// Xray isn't running or the bridge can't be built, fn runs with an empty
// proxyURL so callers fall back to a direct connection.
func (s *NodeService) withOutboundBridge(nodeID int, outboundTag string, fn func(proxyURL string)) {
	proc := XrayProcess()
	if proc == nil || !proc.IsRunning() {
		fn("")
		return
	}
	apiPort := proc.GetAPIPort()
	if apiPort <= 0 {
		fn("")
		return
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fn("")
		return
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	tag := fmt.Sprintf("node-test-%d-%d", nodeID, time.Now().UnixNano())
	proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", port)

	inboundJSON, err := json.Marshal(xray.InboundConfig{
		Listen:   json_util.RawMessage(`"127.0.0.1"`),
		Port:     port,
		Protocol: "socks",
		Settings: json_util.RawMessage(`{"auth":"noauth","udp":false}`),
		Tag:      tag,
	})
	if err != nil {
		fn("")
		return
	}

	cfg := proc.GetConfig()
	routing := map[string]any{}
	if len(cfg.RouterConfig) > 0 {
		_ = json.Unmarshal(cfg.RouterConfig, &routing)
	}
	rules, _ := routing["rules"].([]any)
	rule := map[string]any{
		"type":       "field",
		"inboundTag": []any{tag},
	}
	if routingTagIsBalancer(routing, outboundTag) {
		rule["balancerTag"] = outboundTag
	} else {
		rule["outboundTag"] = outboundTag
	}
	routing["rules"] = append([]any{rule}, rules...)
	routingJSON, err := json.Marshal(routing)
	if err != nil {
		fn("")
		return
	}
	originalRoutingJSON := cfg.RouterConfig

	api := xray.XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		fn("")
		return
	}
	defer api.Close()

	if err := api.AddInbound(inboundJSON); err != nil {
		fn("")
		return
	}
	defer func() {
		if err := api.DelInbound(tag); err != nil {
			logger.Warning("remove temp node bridge inbound failed:", err)
		}
	}()

	if err := api.ApplyRoutingConfig(routingJSON); err != nil {
		fn("")
		return
	}
	defer func() {
		restore := originalRoutingJSON
		if len(restore) == 0 {
			restore = []byte("{}")
		}
		if err := api.ApplyRoutingConfig(restore); err != nil {
			logger.Warning("restore routing after node bridge failed:", err)
		}
	}()

	fn(proxyURL)
}

func (s *NodeService) probe(ctx context.Context, n *model.Node, proxyURL string) (HeartbeatPatch, error) {
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

	client, err := runtime.HTTPClientForNode(n, proxyURL)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}

	start := time.Now()
	resp, err := client.Do(req)
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
				Version  string `json:"version"`
				State    string `json:"state"`
				ErrorMsg string `json:"errorMsg"`
			} `json:"xray"`
			PanelVersion string `json:"panelVersion"`
			PanelGuid    string `json:"panelGuid"`
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
	patch.XrayState = o.Xray.State
	patch.XrayError = o.Xray.ErrorMsg
	patch.PanelVersion = o.PanelVersion
	patch.Guid = o.PanelGuid
	patch.UptimeSecs = o.Uptime
	return patch, nil
}

type ProbeResultUI struct {
	Status       string  `json:"status" example:"online"`
	LatencyMs    int     `json:"latencyMs" example:"42"`
	XrayVersion  string  `json:"xrayVersion" example:"25.10.31"`
	PanelVersion string  `json:"panelVersion" example:"v3.x.x"`
	CpuPct       float64 `json:"cpuPct" example:"12.5"`
	MemPct       float64 `json:"memPct" example:"45.2"`
	UptimeSecs   uint64  `json:"uptimeSecs" example:"86400"`
	Error        string  `json:"error"`
	// XrayState/XrayError are populated on successful probes even when the node's
	// Xray core is not healthy. The UI uses them for a distinct "panel ok, xray failed" indicator.
	XrayState string `json:"xrayState"`
	XrayError string `json:"xrayError"`
}

func (p HeartbeatPatch) ToUI(ok bool) ProbeResultUI {
	r := ProbeResultUI{
		LatencyMs:    p.LatencyMs,
		XrayVersion:  p.XrayVersion,
		PanelVersion: p.PanelVersion,
		CpuPct:       p.CpuPct,
		MemPct:       p.MemPct,
		UptimeSecs:   p.UptimeSecs,
		Error:        FriendlyProbeError(p.LastError),
		XrayState:    p.XrayState,
		XrayError:    p.XrayError,
	}
	if ok {
		r.Status = "online"
	} else {
		r.Status = "offline"
	}
	return r
}

func FriendlyProbeError(msg string) string {
	if strings.Contains(msg, "server gave HTTP response to HTTPS client") {
		return "the server speaks HTTP, not HTTPS; set the node scheme to http"
	}
	return msg
}
