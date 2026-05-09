package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

// remoteHTTPTimeout bounds a single remote API call. Generous enough for
// a slow node under load, short enough that a wedged remote doesn't
// block the central panel's UI thread for the user.
const remoteHTTPTimeout = 10 * time.Second

// remoteHTTPClient is shared so repeated calls to the same node reuse
// connections. Per-request timeouts are set via context.
var remoteHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
	},
}

// envelope mirrors entity.Msg without depending on the entity package
// (avoids a cycle on the controller side that pulls in this runtime).
type envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

// Remote implements Runtime by calling the existing /panel/api/inbounds/*
// endpoints on a remote 3x-ui panel. The remote is authenticated as
// the central panel via its per-node Bearer token.
//
// remoteIDByTag caches the {tag → remote inbound id} mapping so the
// hot path (update/delete/addClient) avoids /list lookups. The cache
// is in-memory and rebuilt lazily on first miss after a process restart
// or InvalidateNode call.
type Remote struct {
	node *model.Node

	mu            sync.RWMutex
	remoteIDByTag map[string]int
}

// NewRemote constructs a Remote runtime for one node. The node pointer
// is cached; callers that mutate node config (via NodeService.Update)
// must drop the runtime through Manager.InvalidateNode so a fresh one
// picks up the new fields.
func NewRemote(n *model.Node) *Remote {
	return &Remote{
		node:          n,
		remoteIDByTag: make(map[string]int),
	}
}

func (r *Remote) Name() string { return "node:" + r.node.Name }

// baseURL composes the panel root for r.node, e.g. https://1.2.3.4:2053/
// Always ends in '/' so callers can append "panel/api/...".
func (r *Remote) baseURL() string {
	bp := r.node.BasePath
	if bp == "" {
		bp = "/"
	}
	if !strings.HasSuffix(bp, "/") {
		bp += "/"
	}
	return fmt.Sprintf("%s://%s:%d%s", r.node.Scheme, r.node.Address, r.node.Port, bp)
}

// do issues an HTTP request against the remote panel and decodes the
// entity.Msg envelope. Returns an error for transport failures, non-2xx
// responses, or {success:false} bodies.
//
// body may be nil. For application/x-www-form-urlencoded calls (the
// existing controllers bind via c.ShouldBind which prefers form-encoded)
// pass url.Values; for JSON pass any other type and we'll marshal it.
func (r *Remote) do(ctx context.Context, method, path string, body any) (*envelope, error) {
	if r.node.ApiToken == "" {
		return nil, errors.New("node has no API token configured")
	}

	target := r.baseURL() + strings.TrimPrefix(path, "/")

	var (
		reqBody     io.Reader
		contentType string
	)
	switch b := body.(type) {
	case nil:
		// nothing
	case url.Values:
		reqBody = strings.NewReader(b.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		buf, err := json.Marshal(b)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
		contentType = "application/json"
	}

	cctx, cancel := context.WithTimeout(ctx, remoteHTTPTimeout)
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

	resp, err := remoteHTTPClient.Do(req)
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

// resolveRemoteID returns the remote panel's local inbound ID for the
// given tag. Cache-backed; on miss it hits /panel/api/inbounds/list and
// repopulates the whole map (one-shot list is cheaper than per-tag
// lookups when several inbounds need resolving in sequence).
func (r *Remote) resolveRemoteID(ctx context.Context, tag string) (int, error) {
	if id, ok := r.cacheGet(tag); ok {
		return id, nil
	}
	if err := r.refreshRemoteIDs(ctx); err != nil {
		return 0, err
	}
	if id, ok := r.cacheGet(tag); ok {
		return id, nil
	}
	return 0, fmt.Errorf("remote inbound with tag %q not found on node %s", tag, r.node.Name)
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

// refreshRemoteIDs replaces the in-memory tag→id map with whatever the
// node currently has. Called on cache miss; also a useful recovery path
// when the remote panel is rebuilt or we get a "not found" on update.
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
	// Strip NodeID from the wire payload so the remote stores a "local"
	// row from its own perspective. We also ship the full model.Inbound
	// minus runtime metadata. Tag is preserved so central + remote agree
	// on the identifier — relies on InboundController being patched to
	// not overwrite a non-empty Tag.
	payload := wireInbound(ib)
	env, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/add", payload)
	if err != nil {
		return err
	}
	// Response body contains the saved inbound (with the remote's Id).
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
		// Already gone on remote — treat as success so a sync after a
		// remote panel reset doesn't strand the central panel.
		logger.Warning("remote DelInbound: tag", ib.Tag, "not found on", r.node.Name, "— treating as success")
		return nil
	}
	if _, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/del/"+strconv.Itoa(id), nil); err != nil {
		return err
	}
	r.cacheDel(ib.Tag)
	return nil
}

func (r *Remote) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {
	// The remote's old row is keyed by oldIb.Tag (tags can change on
	// edit if listen/port changed). We update by remote-id so the row
	// keeps its identity even when its tag flips.
	id, err := r.resolveRemoteID(ctx, oldIb.Tag)
	if err != nil {
		// Remote lost the row — fall back to add. This can happen if
		// the node panel was reset; we'd rather end up with the inbound
		// existing than fail the user's update.
		return r.AddInbound(ctx, newIb)
	}
	payload := wireInbound(newIb)
	if _, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/update/"+strconv.Itoa(id), payload); err != nil {
		return err
	}
	// Tag may have changed — remap the cache.
	if oldIb.Tag != newIb.Tag {
		r.cacheDel(oldIb.Tag)
	}
	r.cacheSet(newIb.Tag, id)
	return nil
}

// AddUser pushes a single client into the remote inbound's settings JSON.
// We can't reuse the central panel's xrayApi.AddUser shape directly
// because the remote's HTTP endpoint expects {id, settings} where
// settings is a JSON string with a "clients":[...] array. The central
// panel's InboundService has already updated its own settings JSON
// before calling us, so we just ship the new full settings to the
// remote via /update — simpler than reconstructing the partial AddUser
// payload remote-side.
//
// Caller passes the full updated *model.Inbound on the same code path
// AddUser is called from in InboundService. To avoid changing the
// Runtime interface for that, AddUser/RemoveUser delegate to UpdateInbound.
func (r *Remote) AddUser(ctx context.Context, ib *model.Inbound, _ map[string]any) error {
	return r.UpdateInbound(ctx, ib, ib)
}

func (r *Remote) RemoveUser(ctx context.Context, ib *model.Inbound, _ string) error {
	return r.UpdateInbound(ctx, ib, ib)
}

func (r *Remote) RestartXray(ctx context.Context) error {
	_, err := r.do(ctx, http.MethodPost, "panel/api/server/restartXrayService", nil)
	return err
}

func (r *Remote) ResetClientTraffic(ctx context.Context, ib *model.Inbound, email string) error {
	id, err := r.resolveRemoteID(ctx, ib.Tag)
	if err != nil {
		// Already gone on remote — central reset is enough.
		logger.Warning("remote ResetClientTraffic: tag", ib.Tag, "not found on", r.node.Name, "— treating as success")
		return nil
	}
	_, err = r.do(ctx, http.MethodPost,
		fmt.Sprintf("panel/api/inbounds/%d/resetClientTraffic/%s", id, url.PathEscape(email)),
		nil)
	return err
}

func (r *Remote) ResetInboundClientTraffics(ctx context.Context, ib *model.Inbound) error {
	id, err := r.resolveRemoteID(ctx, ib.Tag)
	if err != nil {
		logger.Warning("remote ResetInboundClientTraffics: tag", ib.Tag, "not found on", r.node.Name, "— treating as success")
		return nil
	}
	_, err = r.do(ctx, http.MethodPost,
		fmt.Sprintf("panel/api/inbounds/resetAllClientTraffics/%d", id), nil)
	return err
}

func (r *Remote) ResetAllTraffics(ctx context.Context) error {
	_, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/resetAllTraffics", nil)
	return err
}

// TrafficSnapshot is what NodeTrafficSyncJob pulls from a remote node
// every cron tick. Inbounds carry absolute up/down/all_time + ClientStats
// (the same shape /panel/api/inbounds/list returns); the two map fields
// come from the dedicated /onlines and /lastOnline endpoints.
type TrafficSnapshot struct {
	Inbounds      []*model.Inbound
	OnlineEmails  []string
	LastOnlineMap map[string]int64
}

// FetchTrafficSnapshot pulls the three pieces in series. Sequential is
// fine because the cron job already fans out across nodes — adding
// per-node parallelism on top would just thrash the remote.
//
// Not on the Runtime interface: only the sync job needs it, and Local
// has no equivalent (XrayTrafficJob already covers the local engine).
func (r *Remote) FetchTrafficSnapshot(ctx context.Context) (*TrafficSnapshot, error) {
	snap := &TrafficSnapshot{LastOnlineMap: map[string]int64{}}

	envList, err := r.do(ctx, http.MethodGet, "panel/api/inbounds/list", nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(envList.Obj, &snap.Inbounds); err != nil {
		return nil, fmt.Errorf("decode inbound list: %w", err)
	}

	envOnlines, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/onlines", nil)
	if err != nil {
		// Onlines/lastOnline are nice-to-have. A failure here shouldn't
		// invalidate the inbound counter merge — log and continue with
		// empty values, the next tick may succeed.
		logger.Warning("remote", r.node.Name, "onlines fetch failed:", err)
	} else if len(envOnlines.Obj) > 0 {
		_ = json.Unmarshal(envOnlines.Obj, &snap.OnlineEmails)
	}

	envLastOnline, err := r.do(ctx, http.MethodPost, "panel/api/inbounds/lastOnline", nil)
	if err != nil {
		logger.Warning("remote", r.node.Name, "lastOnline fetch failed:", err)
	} else if len(envLastOnline.Obj) > 0 {
		_ = json.Unmarshal(envLastOnline.Obj, &snap.LastOnlineMap)
	}

	return snap, nil
}

// wireInbound builds the request body for /panel/api/inbounds/add and
// /update. Mirrors the form fields the existing InboundController
// expects via c.ShouldBind — we use form-encoded to match exactly.
//
// We deliberately omit Id (remote assigns its own), UserId (remote's
// fallback user takes over), NodeID (the remote sees itself as local),
// and ClientStats (those are joined-table data the remote rebuilds).
func wireInbound(ib *model.Inbound) url.Values {
	v := url.Values{}
	v.Set("up", strconv.FormatInt(ib.Up, 10))
	v.Set("down", strconv.FormatInt(ib.Down, 10))
	v.Set("total", strconv.FormatInt(ib.Total, 10))
	v.Set("remark", ib.Remark)
	v.Set("enable", strconv.FormatBool(ib.Enable))
	v.Set("expiryTime", strconv.FormatInt(ib.ExpiryTime, 10))
	v.Set("listen", ib.Listen)
	v.Set("port", strconv.Itoa(ib.Port))
	v.Set("protocol", string(ib.Protocol))
	v.Set("settings", ib.Settings)
	v.Set("streamSettings", ib.StreamSettings)
	v.Set("tag", ib.Tag)
	v.Set("sniffing", ib.Sniffing)
	if ib.TrafficReset != "" {
		v.Set("trafficReset", ib.TrafficReset)
	}
	return v
}
