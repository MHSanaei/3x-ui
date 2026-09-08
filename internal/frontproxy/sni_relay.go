package frontproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// sniPeekTimeout bounds peekClientHelloSNI; matches Manager's own
// http.Server.ReadHeaderTimeout neighborhood.
const sniPeekTimeout = 4 * time.Second

// errSNIPeekDone aborts the sacrificial handshake once SNI is captured.
var errSNIPeekDone = errors.New("frontproxy: sni peek complete")

// newSNIRelayListener wraps inner so a matching ClientHello's raw bytes get
// spliced to targets[sni] instead of terminated here. Returns inner
// unchanged (no wrapper at all) when targets is empty.
func newSNIRelayListener(inner net.Listener, targets map[string]string) net.Listener {
	if len(targets) == 0 {
		return inner
	}
	lower := make(map[string]string, len(targets))
	for sni, backend := range targets {
		lower[strings.ToLower(sni)] = backend
	}
	l := &sniRelayListener{
		Listener: inner,
		targets:  lower,
		out:      make(chan net.Conn),
		done:     make(chan struct{}),
	}
	go l.acceptLoop()
	return l
}

// sniRelayListener embeds net.Listener; only Accept and Close are
// overridden below.
type sniRelayListener struct {
	net.Listener
	targets map[string]string
	out     chan net.Conn
	done    chan struct{}

	closeOnce sync.Once
	mu        sync.Mutex
	closeErr  error
}

// shutdown marks the listener terminally closed with err. Safe to call more
// than once; only the first call has effect.
func (l *sniRelayListener) shutdown(err error) {
	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.closeErr = err
		l.mu.Unlock()
		close(l.done)
	})
}

// Close stops acceptLoop and unblocks any handle goroutine still trying to
// hand a connection to Accept.
func (l *sniRelayListener) Close() error {
	err := l.Listener.Close()
	l.shutdown(net.ErrClosed)
	return err
}

// acceptLoop hands every accepted connection to its own goroutine
// immediately, so one slow peek can only ever delay itself. A temporary
// Accept error backs off and retries exactly like net/http.Server.Serve.
func (l *sniRelayListener) acceptLoop() {
	var tempDelay time.Duration
	for {
		raw, err := l.Listener.Accept()
		if err != nil {
			select {
			case <-l.done:
				return
			default:
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() { //nolint:staticcheck // mirrors net/http.Server.Serve's own retry logic
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if tempDelay > time.Second {
					tempDelay = time.Second
				}
				logger.Warningf("frontproxy: sni relay: accept: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			l.shutdown(err)
			return
		}
		tempDelay = 0
		go l.handle(raw)
	}
}

// handle peeks raw's SNI and either splices it to a matching backend or
// hands it to Accept exactly as a plain Accept() would have.
func (l *sniRelayListener) handle(raw net.Conn) {
	sni, prefix := peekClientHelloSNI(raw)
	if sni != "" {
		if backend, ok := l.targets[strings.ToLower(sni)]; ok {
			logger.Debugf("frontproxy: sni relay: %q -> %s", sni, backend)
			relayRaw(raw, prefix, backend)
			return
		}
	}
	select {
	case l.out <- &replayConn{Conn: raw, prefix: bytes.NewReader(prefix)}:
	case <-l.done:
		raw.Close()
	}
}

// Accept satisfies net.Listener.
func (l *sniRelayListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.out:
		return c, nil
	case <-l.done:
		l.mu.Lock()
		defer l.mu.Unlock()
		return nil, l.closeErr
	}
}

// peekConn buffers every byte Read (for replay) and discards every Write.
// The discard matters: crypto/tls's readClientHello sends a real
// alertInternalError to the peer before returning GetConfigForClient's
// error, and nothing may reach them during this throwaway handshake.
type peekConn struct {
	net.Conn
	buf bytes.Buffer
}

func (c *peekConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.buf.Write(p[:n])
	}
	return n, err
}

func (c *peekConn) Write(p []byte) (int, error) { return len(p), nil }

// replayConn replays a peeked prefix before falling through to conn, so a
// fresh consumer sees the same bytes a plain Accept() would have given it.
type replayConn struct {
	net.Conn
	prefix *bytes.Reader
}

func (c *replayConn) Read(p []byte) (int, error) {
	if c.prefix.Len() > 0 {
		return c.prefix.Read(p)
	}
	return c.Conn.Read(p)
}

// peekClientHelloSNI extracts the SNI from raw's ClientHello without
// consuming it from the real stream; prefix is what a caller must replay
// first. sni is "" for anything that isn't a complete TLS ClientHello
// within sniPeekTimeout.
func peekClientHelloSNI(raw net.Conn) (sni string, prefix []byte) {
	_ = raw.SetReadDeadline(time.Now().Add(sniPeekTimeout))
	defer func() { _ = raw.SetReadDeadline(time.Time{}) }()
	ctx, cancel := context.WithTimeout(context.Background(), sniPeekTimeout)
	defer cancel()

	pc := &peekConn{Conn: raw}
	cfg := &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			sni = hello.ServerName
			return nil, errSNIPeekDone
		},
	}
	_ = tls.Server(pc, cfg).HandshakeContext(ctx)
	return sni, pc.buf.Bytes()
}

// relayRaw splices raw to backendAddr: prefix goes first, then both
// directions are piped until each independently finishes, half-closing the
// destination as its source direction ends so a half-closed peer's reply
// isn't cut short.
func relayRaw(raw net.Conn, prefix []byte, backendAddr string) {
	defer raw.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	backend, err := (&net.Dialer{}).DialContext(ctx, "tcp", backendAddr)
	if err != nil {
		logger.Warningf("frontproxy: sni relay: dial %s: %v", backendAddr, err)
		return
	}
	defer backend.Close()

	if len(prefix) > 0 {
		if _, err := backend.Write(prefix); err != nil {
			logger.Warningf("frontproxy: sni relay: write ClientHello prefix to %s: %v", backendAddr, err)
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(backend, raw)
		closeWrite(backend)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(raw, backend)
		closeWrite(raw)
	}()
	wg.Wait()
}

// closeWrite half-closes c's write side when the underlying conn supports
// it, so the peer's read sees a clean EOF instead of the whole connection
// dying under a still-flowing reply.
func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}
