package frontproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Options is everything the reverse proxy needs to run, resolved from settings
// by the caller so this package never touches the database.
type Options struct {
	Listen  string
	Port    int
	Routing Config
	Decoy   DecoyConfig
	TLS     TLSSettings
}

// Manager owns the single reverse-proxy listener for the whole install.
type Manager struct {
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	cancel   context.CancelFunc
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide reverse-proxy manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() { manager = &Manager{} })
	return manager
}

// Start brings the reverse proxy up. A no-op, not an error, when it is already
// running -- callers want "make sure it's up", same as the tor manager.
func (m *Manager) Start(opts Options) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server != nil {
		return nil
	}
	if opts.Port <= 0 || opts.Port > 65535 {
		return fmt.Errorf("invalid reverse-proxy port %d", opts.Port)
	}
	if opts.Routing.PanelBasePath == "" || matchesPrefix("/", opts.Routing.PanelBasePath) {
		return fmt.Errorf("the panel needs a non-root base path before the reverse proxy can tell it apart from the decoy")
	}

	ctx, cancel := context.WithCancel(context.Background())
	tlsCfg, err := buildTLSConfig(ctx, opts.TLS)
	if err != nil {
		cancel()
		return fmt.Errorf("reverse-proxy TLS: %w", err)
	}

	listen := opts.Listen
	if listen == "" {
		listen = "127.0.0.1"
	}
	addr := net.JoinHostPort(listen, strconv.Itoa(opts.Port))
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		cancel()
		return fmt.Errorf("reverse-proxy listen on %s: %w", addr, err)
	}
	ln = tls.NewListener(ln, tlsCfg)

	srv := &http.Server{
		Handler:           newHandler(opts.Routing, opts.Decoy),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	m.server, m.listener, m.cancel = srv, ln, cancel
	go func() { _ = srv.Serve(ln) }()
	logger.Infof("frontproxy: listening on %s", addr)
	return nil
}

// Stop shuts the reverse proxy down. A no-op when it is already stopped.
func (m *Manager) Stop() error {
	m.mu.Lock()
	srv, ln, cancel := m.server, m.listener, m.cancel
	m.server, m.listener, m.cancel = nil, nil, nil
	m.mu.Unlock()
	if srv == nil {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	shutdownCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	err := srv.Shutdown(shutdownCtx)
	if ln != nil {
		_ = ln.Close()
	}
	return err
}

// StopAll stops the reverse proxy. Matches the StopAll() shape of the other
// sidecar managers so web.go's shutdown sequence can call it the same way.
func (m *Manager) StopAll() {
	_ = m.Stop()
}

// IsRunning reports whether the reverse proxy is currently listening.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.server != nil
}

// newHandler dispatches each request to the panel, the subscription server,
// or the decoy, per the routing config.
func newHandler(routing Config, decoy DecoyConfig) http.Handler {
	panelProxy := newLoopbackProxy(routing.PanelPort, routing.UpstreamTLS)
	subProxy := newLoopbackProxy(routing.SubPort, routing.UpstreamTLS)
	decoyHandler := newDecoyHandler(decoy)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch routing.resolveTarget(r.URL.Path) {
		case RoutePanel:
			panelProxy.ServeHTTP(w, r)
		case RouteSub:
			subProxy.ServeHTTP(w, r)
		default:
			decoyHandler.ServeHTTP(w, r)
		}
	})
}

// newLoopbackProxy forwards to one of the panel's own listeners. Go's
// ReverseProxy tunnels Upgrade requests itself, which the panel's /ws needs.
//
// useTLS must match how that listener is actually running. A listener with
// certificates wraps itself in an HTTP-to-HTTPS redirector, so a plaintext
// hop there is answered with a 307 back to the URL the client already asked
// for -- an infinite redirect loop rather than an error.
func newLoopbackProxy(port int, useTLS bool) http.Handler {
	scheme := "http"
	var transport http.RoundTripper
	if useTLS {
		scheme = "https"
		// Verifying this certificate is meaningless: it names the public
		// domain, not 127.0.0.1, and the hop never leaves the machine.
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // loopback hop, see above
		}
	}
	target := &url.URL{Scheme: scheme, Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}
	return &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			// SetURL would leave Host pointing at the loopback target; the
			// panel needs the name the client asked for to build its links.
			pr.Out.Host = pr.In.Host
			pr.SetXForwarded()
			// Whatever the hop itself uses, this listener only ever serves
			// TLS, so the client's side was always https.
			pr.Out.Header.Set("X-Forwarded-Proto", "https")
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			logger.Warningf("frontproxy: upstream 127.0.0.1:%d unreachable: %v", port, err)
			w.WriteHeader(http.StatusBadGateway)
		},
	}
}
