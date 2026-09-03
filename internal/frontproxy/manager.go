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
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/network"
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
	// routed is swapped when routing or decoy settings change. The server is
	// handed a stable dispatcher that reads this, so a settings edit does not
	// have to tear down the listener and redo the TLS setup.
	routed atomic.Pointer[http.Handler]
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
	logger.Info("frontproxy: Start: acquiring manager lock")
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server != nil {
		logger.Info("frontproxy: Start: already running, no-op")
		return nil
	}
	if opts.Port <= 0 || opts.Port > 65535 {
		return fmt.Errorf("invalid reverse-proxy port %d", opts.Port)
	}
	if opts.Routing.PanelBasePath == "" || matchesPrefix("/", opts.Routing.PanelBasePath) {
		return fmt.Errorf("the panel needs a non-root base path before the reverse proxy can tell it apart from the decoy")
	}

	logger.Infof("frontproxy: Start: building TLS config (mode=%s)", opts.TLS.Mode)
	ctx, cancel := context.WithCancel(context.Background())
	tlsCfg, err := buildTLSConfig(ctx, opts.TLS)
	if err != nil {
		cancel()
		return fmt.Errorf("reverse-proxy TLS: %w", err)
	}

	logger.Info("frontproxy: Start: TLS config ready, opening listener")
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

	m.store(newHandler(opts.Routing, opts.Decoy))
	srv := &http.Server{
		Handler:           http.HandlerFunc(m.dispatch),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	m.server, m.listener, m.cancel = srv, ln, cancel
	go network.ServeHTTP(srv, ln, "Reverse proxy")
	logger.Infof("frontproxy: listening on %s", addr)
	return nil
}

func (m *Manager) store(h http.Handler) { m.routed.Store(&h) }

// dispatch is the handler the server actually holds. It stays the same object
// for the listener's whole life while what it delegates to can be replaced.
func (m *Manager) dispatch(w http.ResponseWriter, r *http.Request) {
	h := m.routed.Load()
	if h == nil {
		http.Error(w, "", http.StatusServiceUnavailable)
		return
	}
	(*h).ServeHTTP(w, r)
}

// Reload applies changed routing and decoy settings to a running proxy. It
// does not touch the listener, so the port and the certificate keep whatever
// they were given at Start -- those still need a real restart.
func (m *Manager) Reload(opts Options) {
	m.mu.Lock()
	running := m.server != nil
	m.mu.Unlock()
	if !running {
		return
	}
	m.store(newHandler(opts.Routing, opts.Decoy))
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
	// A stopped door has nothing left to finish an in-flight ACME round --
	// without this, an "obtaining" status set right before Stop() would spin
	// in the UI forever with no event left to resolve it.
	resetCertStatus()
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
	// Pooling buys nothing on a hop that never leaves the machine, and a
	// pooled connection reused for a slow request (restartXrayService) has
	// been observed to end in a reset the panel has no chance to log.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	if useTLS {
		scheme = "https"
		// Verifying this certificate is meaningless: it names the public
		// domain, not 127.0.0.1, and the hop never leaves the machine.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // loopback hop, see above
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
