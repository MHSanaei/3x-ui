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
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
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
	// NetUp/NetDown are the node's current interface throughput (bytes/sec),
	// summed over non-virtual interfaces, read from its status response.
	NetUp     uint64
	NetDown   uint64
	LastError string
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
	for _, row := range inboundRows {
		inboundsByNode[row.NodeID] = append(inboundsByNode[row.NodeID], row.Id)
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

	depletedByNode := make(map[int]int)
	disabledByNode := make(map[int]int)
	activeByNode := make(map[int]int)
	statuses, _ := s.nodeClientStatuses()
	seen := make(map[int]map[int]struct{}, len(nodes))
	for _, st := range statuses {
		clientsSeen := seen[st.NodeID]
		if clientsSeen == nil {
			clientsSeen = make(map[int]struct{})
			seen[st.NodeID] = clientsSeen
		}
		if _, dup := clientsSeen[st.ClientID]; dup {
			// A client attached to several inbounds of one node counts once,
			// matching the distinct ClientCount above.
			continue
		}
		clientsSeen[st.ClientID] = struct{}{}
		switch {
		case st.Depleted:
			depletedByNode[st.NodeID]++
		case st.Disabled:
			disabledByNode[st.NodeID]++
		default:
			activeByNode[st.NodeID]++
		}
	}
	onlineByGuid := s.onlineEmailsByGuid()
	selfGuid, _ := (&SettingService{}).GetPanelGuid()
	ambiguous := ambiguousNodeGuids(nodes, selfGuid)
	for _, n := range nodes {
		n.InboundCount = len(inboundsByNode[n.Id])
		n.DepletedCount = depletedByNode[n.Id]
		n.DisabledCount = disabledByNode[n.Id]
		n.ActiveCount = activeByNode[n.Id]
		// Online is attributed to the node that physically hosts the client
		// (by GUID): a client on a sub-node counts under the sub-node, not
		// the intermediate node it syncs through (#4983).
		n.OnlineCount = len(onlineByGuid[effectiveNodeGuid(n, ambiguous)])
	}

	return nodes, nil
}

// nodeClientStatus is one node-hosted client's classification, carrying enough
// identity for callers to bucket it by node id or by attribution GUID.
type nodeClientStatus struct {
	InboundID int
	NodeID    int
	ClientID  int
	Depleted  bool
	Disabled  bool
}

// nodeClientStatuses classifies every client attached to a node-hosted inbound as
// depleted / disabled / active, matching client_traffics by EMAIL rather than by
// inbound_id. client_traffics.inbound_id goes stale after an inbound is
// delete+recreated, so filtering by it silently drops most rows; the
// client_inbounds -> clients join is the reliable client set and the email join
// pulls each client's live counters. Precedence matches the inbound page:
// depleted (expired/exhausted) wins over disabled.
func (s *NodeService) nodeClientStatuses() ([]nodeClientStatus, error) {
	type row struct {
		InboundID  int   `gorm:"column:inbound_id"`
		NodeID     int   `gorm:"column:node_id"`
		ClientID   int   `gorm:"column:client_id"`
		Enable     bool  `gorm:"column:enable"`
		Total      int64 `gorm:"column:total"`
		Up         int64 `gorm:"column:up"`
		Down       int64 `gorm:"column:down"`
		ExpiryTime int64 `gorm:"column:expiry_time"`
	}
	var rows []row
	if err := database.GetDB().Table("inbounds").
		Select("inbounds.id AS inbound_id, inbounds.node_id AS node_id, clients.id AS client_id, " +
			"clients.enable AS enable, ct.total AS total, ct.up AS up, ct.down AS down, ct.expiry_time AS expiry_time").
		Joins("JOIN client_inbounds ON client_inbounds.inbound_id = inbounds.id").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Joins("LEFT JOIN client_traffics ct ON ct.email = clients.email").
		Where("inbounds.node_id IS NOT NULL").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	out := make([]nodeClientStatus, 0, len(rows))
	for _, r := range rows {
		st := nodeClientStatus{InboundID: r.InboundID, NodeID: r.NodeID, ClientID: r.ClientID}
		expired := r.ExpiryTime > 0 && r.ExpiryTime <= now
		exhausted := r.Total > 0 && r.Up+r.Down >= r.Total
		switch {
		case expired || exhausted:
			st.Depleted = true
		case !r.Enable:
			st.Disabled = true
		}
		out = append(out, st)
	}
	return out, nil
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

// effectiveNodeGuid is a node's stable online/inbound attribution key: its
// reported panelGuid, or a master-local synthetic node-id fallback when the node
// has no GUID yet (old build) or its GUID is ambiguous. ambiguous comes from
// ambiguousNodeGuids.
func effectiveNodeGuid(n *model.Node, ambiguous map[string]struct{}) string {
	if n.Guid == "" {
		return synthNodeGuid(n.Id)
	}
	if n.Id > 0 {
		if _, bad := ambiguous[n.Guid]; bad {
			return synthNodeGuid(n.Id)
		}
	}
	return n.Guid
}

// ambiguousNodeGuids returns the panelGuids a node must not be attributed under
// directly, because doing so would merge two distinct identities: a GUID
// reported by more than one of this master's direct nodes (cloned node servers
// ship the same panelGuid in their copied settings), or a GUID equal to the
// master's own panelGuid (a node cloned from the master). A node holding such a
// GUID falls back to its node-unique synthNodeGuid. Transitive sub-nodes (Id 0)
// carry distinct descendant GUIDs by construction and are excluded.
func ambiguousNodeGuids(nodes []*model.Node, selfGuid string) map[string]struct{} {
	counts := make(map[string]int, len(nodes))
	for _, n := range nodes {
		if n.Id > 0 && n.Guid != "" {
			counts[n.Guid]++
		}
	}
	ambiguous := make(map[string]struct{})
	for guid, c := range counts {
		if c > 1 {
			ambiguous[guid] = struct{}{}
		}
	}
	if selfGuid != "" {
		if _, ok := counts[selfGuid]; ok {
			ambiguous[selfGuid] = struct{}{}
		}
	}
	return ambiguous
}

// effectiveNodeKey returns one node's attribution key without a preloaded node
// list — its panelGuid when that GUID uniquely identifies it among the master's
// nodes and differs from the master's own, otherwise its node-unique
// synthNodeGuid. Same rule as effectiveNodeGuid + ambiguousNodeGuids, for the
// write paths that handle a single node (online tree, IP attribution).
func effectiveNodeKey(node *model.Node) string {
	if node == nil {
		return ""
	}
	if node.Guid == "" {
		return synthNodeGuid(node.Id)
	}
	var sameGuid int64
	database.GetDB().Model(&model.Node{}).Where("guid = ?", node.Guid).Count(&sameGuid)
	masterGuid, _ := (&SettingService{}).GetPanelGuid()
	if sameGuid > 1 || node.Guid == masterGuid {
		return synthNodeGuid(node.Id)
	}
	return node.Guid
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
	if n.TlsVerifyMode != "skip" && n.TlsVerifyMode != "pin" && n.TlsVerifyMode != "mtls" {
		n.TlsVerifyMode = "verify"
	}
	if n.TlsVerifyMode == "mtls" && n.Scheme != "https" {
		return common.NewError("mtls requires the node scheme to be https")
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
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		return s.MarkNodeDirtyTx(tx, id)
	}); err != nil {
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
	return s.EnsureInboundTagAllowedTx(database.GetDB(), nodeID, tag)
}

func (s *NodeService) EnsureInboundTagAllowedTx(tx *gorm.DB, nodeID int, tag string) error {
	tag = strings.TrimSpace(tag)
	if nodeID <= 0 || tag == "" {
		return nil
	}
	if tx == nil {
		tx = database.GetDB()
	}
	node := &model.Node{}
	if err := tx.Where("id = ?", nodeID).First(node).Error; err != nil {
		return err
	}
	if node.InboundSyncMode != "selected" {
		return nil
	}
	if slices.Contains(node.InboundTags, tag) {
		return nil
	}
	buf, err := json.Marshal(append(node.InboundTags, tag))
	if err != nil {
		return err
	}
	return tx.Model(model.Node{}).Where("id = ?", nodeID).
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
	// Refuse to delete a node that still owns inbounds: dropping the node row
	// while inbounds keep its node_id leaves orphaned, dangling references that
	// confuse node sync, subscriptions and cleanup. The operator must detach or
	// remove those inbounds first. (DB-002)
	var attached int64
	if err := db.Model(&model.Inbound{}).Where("node_id = ?", id).Count(&attached).Error; err != nil {
		return err
	}
	if attached > 0 {
		return common.NewError(fmt.Sprintf("cannot delete node: %d inbound(s) still attached to it; detach or delete them first", attached))
	}
	// Capture the node's guid before deleting the row so we can drop its per-node
	// IP attribution. NodeClientIp is keyed by the node's attribution key, which
	// is its guid normally but its node-unique key for a cloned/ambiguous-guid
	// node (see effectiveNodeKey) — so we purge both below.
	var guid string
	var n model.Node
	if err := db.Select("guid").Where("id = ?", id).First(&n).Error; err == nil {
		guid = n.Guid
	}
	// Delete the node row and its per-node child rows atomically. Remove the
	// children (traffic baselines, IP attribution) before the parent node row so
	// the ordering already matches a future ON DELETE constraint. Delete stays
	// tolerant of a missing node row so it can still clean up orphaned baselines.
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("node_id = ?", id).Delete(&model.NodeClientTraffic{}).Error; err != nil {
			return err
		}
		guids := []string{synthNodeGuid(id)}
		if guid != "" {
			guids = append(guids, guid)
		}
		if err := tx.Where("node_guid IN ?", guids).Delete(&model.NodeClientIp{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.Node{}).Error
	}); err != nil {
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
func (s *NodeService) UpdatePanels(ids []int, dev bool) ([]NodeUpdateResult, error) {
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
			updErr := remote.UpdatePanel(ctx, dev)
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
		"net_up":         p.NetUp,
		"net_down":       p.NetDown,
		"last_error":     p.LastError,
		"xray_state":     p.XrayState,
		"xray_error":     p.XrayError,
	}
	// Only learn the GUID; never clear a known one if an old-build node (or a
	// failed probe) reports none, so the stable identity survives blips.
	if p.Guid != "" {
		updates["guid"] = p.Guid
		s.warnOnDuplicateGuid(id, p.Guid)
	}
	if err := db.Model(model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	if p.Status == "online" {
		now := time.Unix(p.LastHeartbeat, 0)
		nodeMetrics.append(nodeMetricKey(id, "cpu"), now, p.CpuPct)
		nodeMetrics.append(nodeMetricKey(id, "mem"), now, p.MemPct)
		nodeMetrics.append(nodeMetricKey(id, "netUp"), now, float64(p.NetUp))
		nodeMetrics.append(nodeMetricKey(id, "netDown"), now, float64(p.NetDown))
	}
	return nil
}

// warnedDupGuid remembers the (nodeID -> guid) pairs already warned about so a
// cloned-server collision is logged once, not every heartbeat.
var warnedDupGuid sync.Map

// warnOnDuplicateGuid logs once when a node reports a panelGuid already held by
// another node or by the master itself (the cloned-server footgun). Attribution
// still works — it falls back to node-unique keys — but the operator should
// regenerate the duplicate panelGuid to restore real identity and per-node IP
// attribution. Re-arms if the collision later clears.
func (s *NodeService) warnOnDuplicateGuid(id int, guid string) {
	var clash int64
	database.GetDB().Model(&model.Node{}).Where("guid = ? AND id <> ?", guid, id).Count(&clash)
	masterGuid, _ := (&SettingService{}).GetPanelGuid()
	if clash == 0 && guid != masterGuid {
		warnedDupGuid.Delete(id)
		return
	}
	if prev, ok := warnedDupGuid.Load(id); ok && prev == guid {
		return
	}
	warnedDupGuid.Store(id, guid)
	logger.Warningf("node %d reports panelGuid %s already used by another node or the master (cloned server?) — regenerate it on that node so online and IP attribution stay per-node", id, guid)
}

func (s *NodeService) MarkNodeDirty(id int) error {
	return s.MarkNodeDirtyTx(database.GetDB(), id)
}

func (s *NodeService) MarkNodeDirtyTx(tx *gorm.DB, id int) error {
	if id <= 0 {
		return nil
	}
	if tx == nil {
		return errors.New("nil db transaction")
	}
	return tx.Model(model.Node{}).
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

// IsNodePending reports whether a save targeting this node was deferred because
// the node is unreachable right now — offline or disabled — so the edit only
// reaches it on the next reconcile. It deliberately ignores config_dirty: that
// flag is set on EVERY node-backed edit as the reconcile self-heal marker,
// including edits pushed live to an online node, so keying the user-facing
// "saved, node offline, will sync" toast off it fired the warning on every save
// to a perfectly healthy online node.
func (s *NodeService) IsNodePending(id int) bool {
	enabled, status, _, _, err := s.NodeSyncState(id)
	if err != nil {
		return false
	}
	return !enabled || status != "online"
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

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
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
			NetIO        struct {
				Up   uint64 `json:"up"`
				Down uint64 `json:"down"`
			} `json:"netIO"`
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
	patch.NetUp = o.NetIO.Up
	patch.NetDown = o.NetIO.Down
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
