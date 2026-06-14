package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const remoteHTTPTimeout = 10 * time.Second

type envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

type Remote struct {
	node *model.Node

	mu            sync.RWMutex
	remoteIDByTag map[string]int

	// Per-node client honoring the TLS verify mode, built once and reused; a
	// node config change drops the cached Remote so the next one rebuilds it.
	clientOnce sync.Once
	client     *http.Client
	clientErr  error

	egressResolver NodeEgressResolver
}

type RemoteInboundOption struct {
	Tag      string         `json:"tag"`
	Remark   string         `json:"remark"`
	Protocol model.Protocol `json:"protocol"`
	Port     int            `json:"port"`
}

func NewRemote(n *model.Node, r NodeEgressResolver) *Remote {
	return &Remote{
		node:           n,
		remoteIDByTag:  make(map[string]int),
		egressResolver: r,
	}
}

func (r *Remote) Name() string { return "node:" + r.node.Name }

// httpClient lazily builds and caches the per-node client honoring the TLS
// verify mode, so Remote ops don't fall back to system CA on skip/pin (#5264).
func (r *Remote) httpClient() (*http.Client, error) {
	r.clientOnce.Do(func() {
		proxyURL := ""
		if r.node.OutboundTag != "" && r.egressResolver != nil {
			proxyURL = r.egressResolver.NodeEgressProxyURL(r.node.Id)
		}
		r.client, r.clientErr = HTTPClientForNode(r.node, proxyURL)
	})
	return r.client, r.clientErr
}

func (r *Remote) baseURL() (string, error) {
	addr, err := netsafe.NormalizeHost(r.node.Address)
	if err != nil {
		return "", err
	}
	scheme := r.node.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if r.node.Port <= 0 || r.node.Port > 65535 {
		return "", fmt.Errorf("invalid node port %d", r.node.Port)
	}
	bp := r.node.BasePath
	if bp == "" {
		bp = "/"
	}
	if !strings.HasSuffix(bp, "/") {
		bp += "/"
	}
	u := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(addr, strconv.Itoa(r.node.Port)),
		Path:   bp,
	}
	return u.String(), nil
}

func (r *Remote) do(ctx context.Context, method, path string, body any) (*envelope, error) {
	if r.node.ApiToken == "" {
		return nil, errors.New("node has no API token configured")
	}

	base, err := r.baseURL()
	if err != nil {
		return nil, err
	}
	target := base + strings.TrimPrefix(path, "/")

	var (
		reqBody     io.Reader
		contentType string
	)
	switch b := body.(type) {
	case nil:
	case url.Values:
		reqBody = strings.NewReader(b.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		buf, jerr := json.Marshal(b)
		if jerr != nil {
			return nil, fmt.Errorf("marshal body: %w", jerr)
		}
		reqBody = bytes.NewReader(buf)
		contentType = "application/json"
	}

	cctx, cancel := context.WithTimeout(netsafe.ContextWithAllowPrivate(ctx, r.node.AllowPrivateAddress), remoteHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, method, target, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.node.ApiToken)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	client, err := r.httpClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s: HTTP %d", method, path, resp.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	if !env.Success {
		return &env, fmt.Errorf("remote: %s", env.Msg)
	}
	return &env, nil
}

func (r *Remote) resolveRemoteID(ctx context.Context, tag string) (int, error) {
	if id, ok := r.cacheGetTag(tag); ok {
		return id, nil
	}
	if err := r.refreshRemoteIDs(ctx); err != nil {
		return 0, err
	}
	if id, ok := r.cacheGetTag(tag); ok {
		return id, nil
	}
	return 0, fmt.Errorf("remote inbound with tag %q not found on node %s", tag, r.node.Name)
}

// cacheGetTag looks up a remote inbound id by tag, tolerating an n<id>- prefix
// that lives on only one of the two panels: the node may carry the bare tag
// while the central panel stores the prefixed form, or vice versa.
func (r *Remote) cacheGetTag(tag string) (int, bool) {
	if id, ok := r.cacheGet(tag); ok {
		return id, true
	}
	prefix := fmt.Sprintf("n%d-", r.node.Id)
	if stripped, found := strings.CutPrefix(tag, prefix); found {
		return r.cacheGet(stripped)
	}
	return r.cacheGet(prefix + tag)
}

func (r *Remote) cacheGet(tag string) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.remoteIDByTag[tag]
	return id, ok
}

func (r *Remote) cacheSet(tag string, id int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteIDByTag[tag] = id
}

func (r *Remote) cacheDel(tag string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.remoteIDByTag, tag)
}

func (r *Remote) ListRemoteTags(ctx context.Context) ([]string, error) {
	if err := r.refreshRemoteIDs(ctx); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	tags := make([]string, 0, len(r.remoteIDByTag))
	for tag := range r.remoteIDByTag {
		tags = append(tags, tag)
	}
	return tags, nil
}

func (r *Remote) ListInboundOptions(ctx context.Context) ([]RemoteInboundOption, error) {
	env, err := r.do(ctx, http.MethodGet, "panel/api/inbounds/list", nil)
	if err != nil {
		return nil, err
	}
	var list []RemoteInboundOption
	if err := json.Unmarshal(env.Obj, &list); err != nil {
		return nil, fmt.Errorf("decode inbound list: %w", err)
	}
	return list, nil
}

func (r *Remote) refreshRemoteIDs(ctx context.Context) error {
	env, err := r.do(ctx, http.MethodGet, "panel/api/inbounds/list", nil)
	if err != nil {
		return err
	}
	var list []struct {
		Id  int    `json:"id"`
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal(env.Obj, &list); err != nil {
		return fmt.Errorf("decode inbound list: %w", err)
	}
	next := make(map[string]int, len(list))
	for _, ib := range list {
		if ib.Tag == "" {
			continue
		}
		next[ib.Tag] = ib.Id
	}
	r.mu.Lock()
	r.remoteIDByTag = next
	r.mu.Unlock()
	return nil
}

func (r *Remote) AddInbound(ctx context.Context, ib *model.Inbound) error {
	payload := wireInbound(ib)
	env, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/add", payload)
	if err != nil {
		return err
	}
	var created struct {
		Id  int    `json:"id"`
		Tag string `json:"tag"`
	}
	if len(env.Obj) > 0 {
		if err := json.Unmarshal(env.Obj, &created); err == nil && created.Id > 0 && created.Tag != "" {
			r.cacheSet(created.Tag, created.Id)
		}
	}
	return nil
}

func (r *Remote) DelInbound(ctx context.Context, ib *model.Inbound) error {
	id, err := r.resolveRemoteID(ctx, ib.Tag)
	if err != nil {
		logger.Warning("remote DelInbound: tag", ib.Tag, "not found on", r.node.Name)
		return nil
	}
	if _, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/del/"+strconv.Itoa(id), nil); err != nil {
		return err
	}
	r.cacheDel(ib.Tag)
	return nil
}

func (r *Remote) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {
	id, err := r.resolveRemoteID(ctx, oldIb.Tag)
	if err != nil {
		return r.AddInbound(ctx, newIb)
	}
	payload := wireInbound(newIb)
	if _, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/update/"+strconv.Itoa(id), payload); err != nil {
		return err
	}
	if oldIb.Tag != newIb.Tag {
		r.cacheDel(oldIb.Tag)
	}
	r.cacheSet(newIb.Tag, id)
	return nil
}

func (r *Remote) AddUser(ctx context.Context, ib *model.Inbound, _ map[string]any) error {
	return r.UpdateInbound(ctx, ib, ib)
}

func (r *Remote) RemoveUser(ctx context.Context, ib *model.Inbound, _ string) error {
	return r.UpdateInbound(ctx, ib, ib)
}

func (r *Remote) AddClient(ctx context.Context, ib *model.Inbound, client model.Client) error {
	id, err := r.resolveRemoteID(ctx, ib.Tag)
	if err != nil {
		return fmt.Errorf("remote AddClient: resolve tag %q: %w", ib.Tag, err)
	}
	payload := map[string]any{
		"client":     client,
		"inboundIds": []int{id},
	}
	if _, err := r.do(ctx, http.MethodPost, "panel/api/clients/add", payload); err != nil {
		return err
	}
	return nil
}

func (r *Remote) DeleteUser(ctx context.Context, ib *model.Inbound, email string) error {
	if email == "" {
		return nil
	}
	id, err := r.resolveRemoteID(ctx, ib.Tag)
	if err != nil {
		return nil
	}
	body := map[string]any{"inboundIds": []int{id}}
	_, err = r.do(ctx, http.MethodPost,
		"panel/api/clients/"+url.PathEscape(email)+"/detach", body)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil
	}
	return err
}

func (r *Remote) UpdateUser(ctx context.Context, ib *model.Inbound, oldEmail string, payload model.Client) error {
	if oldEmail == "" {
		oldEmail = payload.Email
	}
	id, err := r.resolveRemoteID(ctx, ib.Tag)
	if err != nil {
		return err
	}
	path := "panel/api/clients/update/" + url.PathEscape(oldEmail) +
		"?inboundIds=" + strconv.Itoa(id)
	if _, err := r.do(ctx, http.MethodPost, path, payload); err != nil {
		return err
	}
	return nil
}

func (r *Remote) RestartXray(ctx context.Context) error {
	_, err := r.do(ctx, http.MethodPost, "panel/api/server/restartXrayService", nil)
	return err
}

// UpdatePanel asks the node to run its own official self-updater (update.sh)
// and restart onto the latest release. The node returns as soon as the job is
// launched; the new version surfaces on the next heartbeat.
func (r *Remote) UpdatePanel(ctx context.Context) error {
	_, err := r.do(ctx, http.MethodPost, "panel/api/server/updatePanel", nil)
	return err
}

// WebCertFiles holds a node's own web TLS certificate and key file paths.
type WebCertFiles struct {
	WebCertFile string `json:"webCertFile"`
	WebKeyFile  string `json:"webKeyFile"`
}

// GetWebCertFiles fetches the node's own web TLS certificate/key file paths so
// the central panel can offer them as the "Set Cert from Panel" default for a
// node-assigned inbound — those paths exist on the node, the central panel's
// don't. See issue #4854.
func (r *Remote) GetWebCertFiles(ctx context.Context) (*WebCertFiles, error) {
	env, err := r.do(ctx, http.MethodGet, "panel/api/server/getWebCertFiles", nil)
	if err != nil {
		return nil, err
	}
	var files WebCertFiles
	if err := json.Unmarshal(env.Obj, &files); err != nil {
		return nil, fmt.Errorf("decode web cert files: %w", err)
	}
	return &files, nil
}

// GetDescendants fetches the node's read-only summaries of the nodes IT
// manages, so this panel can surface them as transitive sub-nodes in a chained
// topology (#4983). Best-effort: an old-build node without the endpoint returns
// an error the caller ignores.
func (r *Remote) GetDescendants(ctx context.Context) ([]model.NodeSummary, error) {
	env, err := r.do(ctx, http.MethodGet, "panel/api/server/descendants", nil)
	if err != nil {
		return nil, err
	}
	var out []model.NodeSummary
	if len(env.Obj) > 0 {
		if err := json.Unmarshal(env.Obj, &out); err != nil {
			return nil, fmt.Errorf("decode descendants: %w", err)
		}
	}
	return out, nil
}

func (r *Remote) ResetClientTraffic(ctx context.Context, _ *model.Inbound, email string) error {
	_, err := r.do(ctx, http.MethodPost,
		"panel/api/clients/resetTraffic/"+url.PathEscape(email), nil)
	return err
}

func (r *Remote) ResetAllTraffics(ctx context.Context) error {
	_, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/resetAllTraffics", nil)
	return err
}

func (r *Remote) ResetInboundTraffic(ctx context.Context, ib *model.Inbound) error {
	_, err := r.do(ctx, http.MethodPost, fmt.Sprintf("panel/api/inbounds/%d/resetTraffic", ib.Id), nil)
	return err
}

type TrafficSnapshot struct {
	Inbounds     []*model.Inbound
	OnlineEmails []string
	// OnlineTree is the node's GUID-keyed online subtree (its own clients under
	// its panelGuid plus every descendant under theirs). Preferred over the flat
	// OnlineEmails so the master can attribute deeply nested clients to the real
	// node across a chain (#4983). Empty when the node is an old build without
	// the per-GUID endpoint — OnlineEmails is the fallback then.
	OnlineTree    map[string][]string
	LastOnlineMap map[string]int64
}

func (r *Remote) FetchTrafficSnapshot(ctx context.Context) (*TrafficSnapshot, error) {
	snap := &TrafficSnapshot{LastOnlineMap: map[string]int64{}}

	envList, err := r.do(ctx, http.MethodGet, "panel/api/inbounds/list", nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(envList.Obj, &snap.Inbounds); err != nil {
		return nil, fmt.Errorf("decode inbound list: %w", err)
	}

	// Prefer the GUID-keyed subtree; fall back to the flat list only when the
	// node is an old build without the per-GUID endpoint (#4983).
	envTree, err := r.do(ctx, http.MethodPost, "panel/api/clients/onlinesByGuid", nil)
	if err == nil && len(envTree.Obj) > 0 {
		_ = json.Unmarshal(envTree.Obj, &snap.OnlineTree)
	}
	if len(snap.OnlineTree) == 0 {
		envOnlines, err := r.do(ctx, http.MethodPost, "panel/api/clients/onlines", nil)
		if err != nil {
			logger.Warning("remote", r.node.Name, "onlines fetch failed:", err)
		} else if len(envOnlines.Obj) > 0 {
			_ = json.Unmarshal(envOnlines.Obj, &snap.OnlineEmails)
		}
	}

	envLastOnline, err := r.do(ctx, http.MethodPost, "panel/api/clients/lastOnline", nil)
	if err != nil {
		logger.Warning("remote", r.node.Name, "lastOnline fetch failed:", err)
	} else if len(envLastOnline.Obj) > 0 {
		_ = json.Unmarshal(envLastOnline.Obj, &snap.LastOnlineMap)
	}

	return snap, nil
}

// PushGlobalClientTraffics sends this panel's aggregated per-client usage to
// the node, tagged with this panel's GUID so the node keeps one row per
// pushing master. Display/enforcement input on the node only — the node never
// folds these into the counters it reports back, so this panel's (and any
// other master's) delta accounting over the node snapshot stays intact.
func (r *Remote) PushGlobalClientTraffics(ctx context.Context, masterGuid string, traffics []*xray.ClientTraffic) error {
	payload := map[string]any{
		"masterGuid": masterGuid,
		"traffics":   traffics,
	}
	_, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/pushClientTraffics", payload)
	return err
}

func wireInbound(ib *model.Inbound) url.Values {
	v := url.Values{}
	v.Set("total", strconv.FormatInt(ib.Total, 10))
	v.Set("remark", ib.Remark)
	v.Set("subSortIndex", strconv.Itoa(ib.SubSortIndex))
	v.Set("enable", strconv.FormatBool(ib.Enable))
	v.Set("expiryTime", strconv.FormatInt(ib.ExpiryTime, 10))
	v.Set("listen", ib.Listen)
	v.Set("port", strconv.Itoa(ib.Port))
	v.Set("protocol", string(ib.Protocol))
	v.Set("settings", ib.Settings)
	v.Set("streamSettings", sanitizeStreamSettingsForRemote(ib.StreamSettings))
	v.Set("tag", ib.Tag)
	v.Set("sniffing", ib.Sniffing)
	shareAddrStrategy := strings.TrimSpace(ib.ShareAddrStrategy)
	switch shareAddrStrategy {
	case "listen", "custom":
	default:
		shareAddrStrategy = "node"
	}
	v.Set("shareAddrStrategy", shareAddrStrategy)
	v.Set("shareAddr", ib.ShareAddr)
	if ib.TrafficReset != "" {
		v.Set("trafficReset", ib.TrafficReset)
	}
	return v
}

// sanitizeStreamSettingsForRemote strips file-based TLS certificate paths
// from the StreamSettings before sending to a remote node, but ONLY when
// inline certificate content (certificate / key) is also present in the same
// entry.  In that case the file paths are redundant and stripping them avoids
// confusion when the central panel's local paths don't exist on the remote.
//
// When a certificate entry contains ONLY file paths (no inline content) the
// paths are left untouched: the user explicitly entered paths that exist on
// the remote node's filesystem, and removing them would leave Xray with TLS
// configured but no certificate, causing Xray to crash on the remote node.
func sanitizeStreamSettingsForRemote(streamSettings string) string {
	if streamSettings == "" {
		return streamSettings
	}

	var stream map[string]any
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return streamSettings
	}

	tlsSettings, ok := stream["tlsSettings"].(map[string]any)
	if !ok {
		return streamSettings
	}

	certificates, ok := tlsSettings["certificates"].([]any)
	if !ok {
		return streamSettings
	}

	changed := false
	for _, cert := range certificates {
		c, ok := cert.(map[string]any)
		if !ok {
			continue
		}
		// Only strip file paths when inline content is present so that the
		// remote Xray still has a valid certificate to use.
		hasCertFile := c["certificateFile"] != nil && c["certificateFile"] != ""
		hasKeyFile := c["keyFile"] != nil && c["keyFile"] != ""
		hasCertInline := isNonEmptySlice(c["certificate"])
		hasKeyInline := isNonEmptySlice(c["key"])
		if hasCertFile && hasCertInline {
			delete(c, "certificateFile")
			changed = true
		}
		if hasKeyFile && hasKeyInline {
			delete(c, "keyFile")
			changed = true
		}
	}

	if !changed {
		return streamSettings
	}
	out, err := json.Marshal(stream)
	if err != nil {
		return streamSettings
	}
	return string(out)
}

// isNonEmptySlice reports whether v is a non-nil, non-empty JSON array value.
func isNonEmptySlice(v any) bool {
	s, ok := v.([]any)
	return ok && len(s) > 0
}

func (r *Remote) FetchAllClientIps(ctx context.Context) ([]model.InboundClientIps, error) {
	env, err := r.do(ctx, http.MethodGet, "panel/api/server/clientIps", nil)
	if err != nil {
		return nil, err
	}
	var ips []model.InboundClientIps
	if len(env.Obj) > 0 {
		if err := json.Unmarshal(env.Obj, &ips); err != nil {
			return nil, fmt.Errorf("decode client ips: %w", err)
		}
	}
	return ips, nil
}

func (r *Remote) PushAllClientIps(ctx context.Context, ips []model.InboundClientIps) error {
	_, err := r.do(ctx, http.MethodPost, "panel/api/server/clientIps", ips)
	return err
}
