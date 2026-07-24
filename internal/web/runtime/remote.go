package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/mhsanaei/3x-ui/v3/internal/util/wirecodec"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const remoteHTTPTimeout = 10 * time.Second

// zstdMinBodyBytes is the smallest body worth compressing; below it the framing
// overhead can outweigh the savings.
const zstdMinBodyBytes = 1024

// maxRemoteResponseBytes caps a single node RPC's response body. It bounds the
// wire/decompressed size of one response — the real guard against a broken or
// hostile node streaming an unbounded body. It is NOT a process-wide memory
// bound: concurrent RPCs and the decoded JSON can each exceed it, so
// endpoint-specific caps and a concurrency budget remain follow-ups. Node
// responses (traffic snapshots, client-IP lists, inbound options) are JSON and
// stay well under it.
const maxRemoteResponseBytes = 64 << 20 // 64 MiB

// errBodyDiagBytes bounds how much of a non-OK error body we read for a
// diagnostic snippet (and to let small-error connections be reused) without
// buffering a potentially huge or hostile error payload.
const errBodyDiagBytes = 8 << 10 // 8 KiB

// errRemoteResponseTooLarge is returned when a node response exceeds the cap.
var errRemoteResponseTooLarge = errors.New("remote response exceeds size limit")

// readCappedBody reads all of r but rejects bodies larger than limit, returning
// errRemoteResponseTooLarge. It reads at most limit+1 bytes so a body of exactly
// limit is accepted and the first oversize byte is detected without buffering
// more.
func readCappedBody(r io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errRemoteResponseTooLarge
	}
	return raw, nil
}

type envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

// remoteAPIError is a node-panel envelope failure (HTTP 200, success=false),
// distinct from transport/HTTP-status errors so callers can trust its message.
type remoteAPIError struct{ msg string }

func (e *remoteAPIError) Error() string { return "remote: " + e.msg }

type Remote struct {
	node *model.Node

	mu            sync.RWMutex
	remoteIDByTag map[string]int
	// pushedFP holds the fingerprint of the last inbound wire payload successfully
	// pushed, keyed by panel-side tag, so reconcile can skip re-sending an
	// unchanged inbound. Guarded by mu; dropped with the Remote on node config change.
	pushedFP map[string]string
	// supportsZstd is learned from the node's X-3x-Node-Caps response header; once
	// seen, config pushes to this node are zstd-compressed. Old nodes never set
	// it, so they keep receiving plain bodies (mixed-version safe).
	supportsZstd bool

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
		pushedFP:       make(map[string]string),
		egressResolver: r,
	}
}

func (r *Remote) Name() string { return "node:" + r.node.Name }

func (r *Remote) nodeSupportsZstd() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.supportsZstd
}

// recordCaps learns the node's capabilities from a response header so later
// pushes can use the negotiated envelope.
func (r *Remote) recordCaps(h http.Header) {
	if !strings.Contains(h.Get(wirecodec.CapsHeader), wirecodec.CapZstd) {
		return
	}
	r.mu.Lock()
	r.supportsZstd = true
	r.mu.Unlock()
}

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
	// mtls nodes authenticate via the client certificate, so a bearer token is
	// optional for them; every other mode still requires one.
	if r.node.ApiToken == "" && r.node.TlsVerifyMode != "mtls" {
		return nil, errors.New("node has no API token configured")
	}

	base, err := r.baseURL()
	if err != nil {
		return nil, err
	}
	target := base + strings.TrimPrefix(path, "/")

	var (
		bodyBytes   []byte
		contentType string
	)
	switch b := body.(type) {
	case nil:
	case url.Values:
		bodyBytes = []byte(b.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		buf, jerr := json.Marshal(b)
		if jerr != nil {
			return nil, fmt.Errorf("marshal body: %w", jerr)
		}
		bodyBytes = buf
		contentType = "application/json"
	}

	// Attach the integrity hash of the uncompressed body unconditionally (a new
	// node verifies it, an old one ignores it), and zstd-compress only when the
	// node advertised support and the body is worth it.
	var (
		reqBody     io.Reader
		hashHex     string
		zstdEncoded bool
	)
	if bodyBytes != nil {
		hashHex = wirecodec.Sha256Hex(bodyBytes)
		if len(bodyBytes) >= zstdMinBodyBytes && r.nodeSupportsZstd() {
			bodyBytes = wirecodec.Compress(bodyBytes)
			zstdEncoded = true
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	cctx, cancel := context.WithTimeout(netsafe.ContextWithAllowPrivate(ctx, r.node.AllowPrivateAddress), remoteHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, method, target, reqBody)
	if err != nil {
		return nil, err
	}
	if r.node.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.node.ApiToken)
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if hashHex != "" {
		req.Header.Set(wirecodec.HashHeader, hashHex)
	}
	if zstdEncoded {
		req.Header.Set("Content-Encoding", wirecodec.EncodingZstd)
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
	r.recordCaps(resp.Header)

	// Validate status before reading a success payload: a non-OK response's
	// body is never used beyond a short diagnostic, so don't let a node force us
	// to buffer a large body just to return an HTTP error.
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, errBodyDiagBytes))
		if msg := bytes.TrimSpace(snippet); len(msg) > 0 {
			// %q quotes/escapes the untrusted node body so control characters or
			// newlines in it can't garble or inject into the error/log output.
			return nil, fmt.Errorf("%s %s: HTTP %d: %q", method, path, resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("%s %s: HTTP %d", method, path, resp.StatusCode)
	}

	// Fast-fail on an honestly-declared oversize body; the LimitReader below is
	// the real guard since Content-Length is untrusted, may be absent, or is -1
	// under transparent decompression.
	if resp.ContentLength > maxRemoteResponseBytes {
		return nil, fmt.Errorf("%s %s: %w (content-length %d, cap %d)", method, path, errRemoteResponseTooLarge, resp.ContentLength, maxRemoteResponseBytes)
	}

	raw, err := readCappedBody(resp.Body, maxRemoteResponseBytes)
	if err != nil {
		if errors.Is(err, errRemoteResponseTooLarge) {
			return nil, fmt.Errorf("%s %s: %w (cap %d bytes)", method, path, err, maxRemoteResponseBytes)
		}
		return nil, fmt.Errorf("read body: %w", err)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	if !env.Success {
		return &env, &remoteAPIError{msg: env.Msg}
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

// nodeInboundTagPrefix is the central-panel alias for an inbound on nodeID.
// Kept in sync with service.nodeTagPrefix (port_conflict.go); duplicated here
// so runtime does not import service.
func nodeInboundTagPrefix(nodeID int) string {
	return fmt.Sprintf("n%d-", nodeID)
}

// stripNodeInboundTagPrefix removes the central-only n<id>- prefix before
// pushing an inbound to the node so Xray keeps its original tag and routing.
func stripNodeInboundTagPrefix(nodeID int, tag string) string {
	if stripped, ok := strings.CutPrefix(tag, nodeInboundTagPrefix(nodeID)); ok {
		return stripped
	}
	return tag
}

// cacheGetTag looks up a remote inbound id by tag, tolerating an n<id>- prefix
// that lives on only one of the two panels: the node may carry the bare tag
// while the central panel stores the prefixed form, or vice versa.
func (r *Remote) cacheGetTag(tag string) (int, bool) {
	if id, ok := r.cacheGet(tag); ok {
		return id, true
	}
	prefix := nodeInboundTagPrefix(r.node.Id)
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
	delete(r.pushedFP, tag)
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
	payload := wireInbound(ib, r.node.Id)
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
	r.recordPushedInbound(ib)
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
	payload := wireInbound(newIb, r.node.Id)
	if _, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/update/"+strconv.Itoa(id), payload); err != nil {
		return err
	}
	if oldIb.Tag != newIb.Tag {
		r.cacheDel(oldIb.Tag)
	}
	r.cacheSet(newIb.Tag, id)
	r.recordPushedInbound(newIb)
	return nil
}

// ReconcileInbound pushes ib only when its wire payload differs from the last
// successful push, or when the node no longer reports the tag (existsOnNode
// false) — a node that dropped/restarted must still be re-seeded. Returns
// whether a push actually happened. This turns a full-fleet reconcile from "send
// every inbound's full settings" into "send only what changed".
func (r *Remote) ReconcileInbound(ctx context.Context, ib *model.Inbound, existsOnNode bool) (bool, error) {
	fp := wireFingerprint(wireInbound(ib, r.node.Id))
	if existsOnNode {
		r.mu.RLock()
		prev, ok := r.pushedFP[ib.Tag]
		r.mu.RUnlock()
		if ok && prev == fp {
			return false, nil
		}
	}
	if err := r.UpdateInbound(ctx, ib, ib); err != nil {
		return false, err
	}
	return true, nil
}

// recordPushedInbound stamps the fingerprint after a full-payload push — the
// only operation that proves the node holds the entire wire payload.
func (r *Remote) recordPushedInbound(ib *model.Inbound) {
	fp := wireFingerprint(wireInbound(ib, r.node.Id))
	r.mu.Lock()
	r.pushedFP[ib.Tag] = fp
	r.mu.Unlock()
}

// RecordAdoptedInbound stamps the fingerprint when the master adopts the
// node's own settings serialization into its DB — direct knowledge of the
// exact payload the node holds.
func (r *Remote) RecordAdoptedInbound(ib *model.Inbound) {
	r.recordPushedInbound(ib)
}

// AdvancePushedInbound moves the reconcile-skip fingerprint from an inbound's
// pre-edit payload to its post-edit payload once every per-client push for the
// edit succeeded. It advances only when the recorded fingerprint proves the
// node held the exact pre-edit state; otherwise the stale fingerprint stays and
// the next reconcile re-sends the full inbound.
func (r *Remote) AdvancePushedInbound(prevIb, ib *model.Inbound) {
	prevFP := wireFingerprint(wireInbound(prevIb, r.node.Id))
	nextFP := wireFingerprint(wireInbound(ib, r.node.Id))
	r.mu.Lock()
	if r.pushedFP[ib.Tag] == prevFP {
		r.pushedFP[ib.Tag] = nextFP
	}
	r.mu.Unlock()
}

// wireFingerprint hashes a wire payload so an unchanged inbound is cheap to detect.
func wireFingerprint(v url.Values) string {
	sum := sha256.Sum256([]byte(v.Encode()))
	return hex.EncodeToString(sum[:])
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
		// Can't confirm the delete reached the node — surface it so the caller
		// marks the node dirty and a reconcile converges, instead of silently
		// dropping the delete and letting the next snapshot resurrect the client.
		return fmt.Errorf("remote DeleteUser: resolve tag %q: %w", ib.Tag, err)
	}
	body := map[string]any{"inboundIds": []int{id}}
	_, err = r.do(ctx, http.MethodPost,
		"panel/api/clients/"+url.PathEscape(email)+"/detach", body)
	if err == nil {
		return nil
	}
	var apiErr *remoteAPIError
	if errors.As(err, &apiErr) && strings.Contains(strings.ToLower(apiErr.msg), "not found") {
		return nil
	}
	return err
}

func (r *Remote) DeleteClient(ctx context.Context, email string) error {
	if email == "" {
		return nil
	}
	_, err := r.do(ctx, http.MethodPost,
		"panel/api/clients/del/"+url.PathEscape(email), nil)
	if err == nil {
		return nil
	}
	var apiErr *remoteAPIError
	if errors.As(err, &apiErr) && strings.Contains(strings.ToLower(apiErr.msg), "not found") {
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
// launched; the new version surfaces on the next heartbeat. When dev is true the
// node is moved to the rolling dev channel instead of the latest stable release.
func (r *Remote) UpdatePanel(ctx context.Context, dev bool) error {
	var body any
	if dev {
		body = url.Values{"dev": {"true"}}
	}
	_, err := r.do(ctx, http.MethodPost, "panel/api/server/updatePanel", body)
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
	// HostGroups carries the node's per-inbound host overrides (TLS/SNI/
	// fingerprint), fetched only when the snapshot holds a not-yet-adopted tag.
	HostGroups []*entity.HostGroup
}

// FetchHostGroups pulls the node's host overrides so a freshly adopted inbound
// keeps its subscription TLS/SNI/fingerprint settings on the master.
func (r *Remote) FetchHostGroups(ctx context.Context) ([]*entity.HostGroup, error) {
	env, err := r.do(ctx, http.MethodGet, "panel/api/hosts/list", nil)
	if err != nil {
		return nil, err
	}
	var groups []*entity.HostGroup
	if len(env.Obj) > 0 {
		if err := json.Unmarshal(env.Obj, &groups); err != nil {
			return nil, fmt.Errorf("decode host groups: %w", err)
		}
	}
	return groups, nil
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

func wireInbound(ib *model.Inbound, remoteNodeID int) url.Values {
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
	tag := ib.Tag
	if remoteNodeID > 0 {
		tag = stripNodeInboundTagPrefix(remoteNodeID, tag)
	}
	v.Set("tag", tag)
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

// FetchClientIpsByGuid pulls the node's per-node IP attribution subtree
// (guid -> email -> observed IPs). Unlike FetchAllClientIps (the flat union the
// master also pushes back), this preserves which physical node each IP is on.
// Returns an empty map for older nodes that lack the endpoint.
func (r *Remote) FetchClientIpsByGuid(ctx context.Context) (map[string]map[string][]model.ClientIpEntry, error) {
	env, err := r.do(ctx, http.MethodPost, "panel/api/clients/clientIpsByGuid", nil)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string][]model.ClientIpEntry{}
	if len(env.Obj) > 0 {
		if err := json.Unmarshal(env.Obj, &out); err != nil {
			return nil, fmt.Errorf("decode client ips by guid: %w", err)
		}
	}
	return out, nil
}
