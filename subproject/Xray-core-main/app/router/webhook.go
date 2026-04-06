package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/features/routing"
	routing_session "github.com/xtls/xray-core/features/routing/session"
)

// parseURL splits a webhook URL into an HTTP URL and an optional Unix socket
// path. For regular http/https URLs the input is returned unchanged with an
// empty socketPath. For Unix sockets the format is:
//
//	/path/to/socket.sock:/http/path
//	@abstract:/http/path
//	@@padded:/http/path
//
// The :/ separator after the socket path delimits the HTTP request path.
// If omitted, "/" is used.
func parseURL(raw string) (httpURL, socketPath string) {
	if len(raw) == 0 || (!filepath.IsAbs(raw) && raw[0] != '@') {
		return raw, ""
	}
	if idx := strings.Index(raw, ":/"); idx >= 0 {
		return "http://localhost" + raw[idx+1:], raw[:idx]
	}
	return "http://localhost/", raw
}

// resolveSocketPath applies platform-specific transformations to a Unix
// socket path, matching the behaviour of the listen side in
// transport/internet/system_listener.go.
//
// For abstract sockets (prefix @) on Linux/Android:
//   - single @ — used as-is (lock-free abstract socket)
//   - double @@ — stripped to single @ and padded to
//     syscall.RawSockaddrUnix{}.Path length (HAProxy compat)
func resolveSocketPath(path string) string {
	if len(path) == 0 || path[0] != '@' {
		return path
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "android" {
		return path
	}
	if len(path) > 1 && path[1] == '@' {
		fullAddr := make([]byte, len(syscall.RawSockaddrUnix{}.Path))
		copy(fullAddr, path[1:])
		return string(fullAddr)
	}
	return path
}

func ptr[T any](v T) *T { return &v }

type event struct {
	Email          *string `json:"email"`
	Level          *uint32 `json:"level"`
	Protocol       *string `json:"protocol"`
	Network        *string `json:"network"`
	Source         *string `json:"source"`
	Destination    *string `json:"destination"`
	OriginalTarget *string `json:"originalTarget"`
	RouteTarget    *string `json:"routeTarget"`
	InboundTag     *string `json:"inboundTag"`
	InboundName    *string `json:"inboundName"`
	InboundLocal   *string `json:"inboundLocal"`
	OutboundTag    *string `json:"outboundTag"`
	Timestamp      int64   `json:"ts"`
}

type WebhookNotifier struct {
	url           string
	headers       map[string]string
	deduplication uint32
	client        *http.Client
	seen          sync.Map
	done          chan struct{}
	wg            sync.WaitGroup
	closeOnce     sync.Once
}

func NewWebhookNotifier(cfg *WebhookConfig) (*WebhookNotifier, error) {
	if cfg == nil || cfg.Url == "" {
		return nil, nil
	}

	httpURL, socketPath := parseURL(cfg.Url)
	h := &WebhookNotifier{
		url:           httpURL,
		deduplication: cfg.Deduplication,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		done: make(chan struct{}),
	}

	if socketPath != "" {
		dialAddr := resolveSocketPath(socketPath)
		h.client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", dialAddr)
			},
		}
	}

	if len(cfg.Headers) > 0 {
		h.headers = make(map[string]string, len(cfg.Headers))
		for k, v := range cfg.Headers {
			h.headers[k] = v
		}
	}

	if h.deduplication > 0 {
		h.wg.Add(1)
		go h.cleanupLoop()
	}

	return h, nil
}

func (h *WebhookNotifier) Fire(ctx routing.Context, outboundTag string) {
	ev := buildEvent(ctx, outboundTag)

	email := ""
	if ev.Email != nil {
		email = *ev.Email
	}
	if h.isDuplicate(email) {
		return
	}

	h.wg.Add(1)
	select {
	case <-h.done:
		h.wg.Done()
		return
	default:
	}
	go func() {
		defer h.wg.Done()
		h.post(ev)
	}()
}

func buildEvent(ctx routing.Context, outboundTag string) *event {
	ev := &event{
		Timestamp:   time.Now().Unix(),
		OutboundTag: ptr(outboundTag),
		InboundTag:  ptr(ctx.GetInboundTag()),
		Protocol:    ptr(ctx.GetProtocol()),
		Network:     ptr(ctx.GetNetwork().SystemString()),
	}

	if user := ctx.GetUser(); user != "" {
		ev.Email = ptr(user)
	}

	if srcIPs := ctx.GetSourceIPs(); len(srcIPs) > 0 {
		srcPort := ctx.GetSourcePort()
		ev.Source = ptr(net.JoinHostPort(srcIPs[0].String(), srcPort.String()))
	}

	targetPort := ctx.GetTargetPort()
	if domain := ctx.GetTargetDomain(); domain != "" {
		ev.Destination = ptr(net.JoinHostPort(domain, targetPort.String()))
	} else if targetIPs := ctx.GetTargetIPs(); len(targetIPs) > 0 {
		ev.Destination = ptr(net.JoinHostPort(targetIPs[0].String(), targetPort.String()))
	}

	if localIPs := ctx.GetLocalIPs(); len(localIPs) > 0 {
		localPort := ctx.GetLocalPort()
		ev.InboundLocal = ptr(net.JoinHostPort(localIPs[0].String(), localPort.String()))
	}

	if sctx, ok := ctx.(*routing_session.Context); ok {
		enrichFromSession(ev, sctx)
	}

	return ev
}

func enrichFromSession(ev *event, sctx *routing_session.Context) {
	if sctx.Inbound != nil {
		ev.InboundName = ptr(sctx.Inbound.Name)
		if sctx.Inbound.User != nil {
			ev.Level = ptr(sctx.Inbound.User.Level)
		}
	}
	if sctx.Outbound != nil {
		if sctx.Outbound.OriginalTarget.Address != nil {
			ev.OriginalTarget = ptr(sctx.Outbound.OriginalTarget.String())
		}
		if sctx.Outbound.RouteTarget.Address != nil {
			ev.RouteTarget = ptr(sctx.Outbound.RouteTarget.String())
		}
	}
}

func (h *WebhookNotifier) post(ev *event) {
	body, err := json.Marshal(ev)
	if err != nil {
		errors.LogWarning(context.Background(), "webhook: marshal failed: ", err)
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		errors.LogWarning(context.Background(), "webhook: request build failed: ", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range h.headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		errors.LogInfo(context.Background(), "webhook: POST failed: ", err)
		return
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode >= 400 {
		errors.LogWarning(context.Background(), "webhook: POST returned status ", resp.StatusCode)
	}
}

func (h *WebhookNotifier) isDuplicate(email string) bool {
	if h.deduplication == 0 || email == "" {
		return false
	}
	ttl := time.Duration(h.deduplication) * time.Second
	now := time.Now()
	if v, loaded := h.seen.LoadOrStore(email, now); loaded {
		if now.Sub(v.(time.Time)) < ttl {
			return true
		}
		h.seen.Store(email, now)
	}
	return false
}

func (h *WebhookNotifier) cleanupLoop() {
	defer h.wg.Done()
	ttl := time.Duration(h.deduplication) * time.Second
	ticker := time.NewTicker(ttl)
	defer ticker.Stop()
	for {
		select {
		case <-h.done:
			return
		case <-ticker.C:
			now := time.Now()
			h.seen.Range(func(key, value any) bool {
				if now.Sub(value.(time.Time)) >= ttl {
					h.seen.Delete(key)
				}
				return true
			})
		}
	}
}

func (h *WebhookNotifier) Close() error {
	h.closeOnce.Do(func() {
		close(h.done)
	})
	h.wg.Wait()
	h.client.CloseIdleConnections()
	return nil
}
