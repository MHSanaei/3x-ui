package tor

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const controlDialTimeout = 5 * time.Second

// controlAddr is a var (not a plain fmt.Sprintf call inline) so tests can
// point NewIdentity at a fake local control server instead of a real tor.
var controlAddr = fmt.Sprintf("127.0.0.1:%d", ControlPort)

func dialControl() (net.Conn, error) {
	return net.DialTimeout("tcp", controlAddr, controlDialTimeout)
}

func controlSend(conn net.Conn, line string) error {
	_ = conn.SetWriteDeadline(time.Now().Add(controlDialTimeout))
	_, err := conn.Write([]byte(line + "\r\n"))
	return err
}

func controlExpectOK(r *bufio.Reader) error {
	line, err := r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading control response: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "250") {
		return fmt.Errorf("tor control error: %s", line)
	}
	return nil
}

// controlAuthenticate reads the cookie Tor wrote under DataDirectory and
// authenticates with it -- the standard, password-free way to talk to a
// loopback-only control port that only the same OS user can ever read.
func controlAuthenticate(conn net.Conn, r *bufio.Reader) error {
	cookie, err := os.ReadFile(controlCookiePath())
	if err != nil {
		return fmt.Errorf("reading control auth cookie: %w", err)
	}
	if err := controlSend(conn, "AUTHENTICATE "+hex.EncodeToString(cookie)); err != nil {
		return err
	}
	return controlExpectOK(r)
}

// NewIdentity asks Tor to stop using its current circuits for new streams
// (SIGNAL NEWNYM) -- the same "New Identity" action Tor Browser exposes.
// Already-open connections are unaffected, and Tor itself rate-limits how
// often this actually takes effect (about once per 10s), so calling it
// again shortly after may be a no-op on Tor's side even though this
// function itself succeeds.
func NewIdentity() error {
	conn, err := dialControl()
	if err != nil {
		return fmt.Errorf("connecting to tor control port: %w", err)
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	if err := controlAuthenticate(conn, r); err != nil {
		return err
	}
	if err := controlSend(conn, "SIGNAL NEWNYM"); err != nil {
		return err
	}
	return controlExpectOK(r)
}

// socksHTTPClient builds an http.Client that dials out through the managed
// SOCKS proxy.
func socksHTTPClient(timeout time.Duration) (*http.Client, error) {
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", SocksPort), nil, proxy.Direct)
	if err != nil {
		return nil, err
	}
	ctxDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("SOCKS5 dialer does not support context")
	}
	return &http.Client{
		Transport: &http.Transport{DialContext: ctxDialer.DialContext},
		Timeout:   timeout,
	}, nil
}

// CurrentIP dials out through the managed SOCKS proxy to check.torproject.org's
// own API, which confirms both the current exit IP and that traffic is
// genuinely exiting via Tor -- not, say, silently falling through to a
// direct connection.
func CurrentIP(ctx context.Context) (ip string, isTor bool, err error) {
	client, err := socksHTTPClient(30 * time.Second)
	if err != nil {
		return "", false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://check.torproject.org/api/ip", nil)
	if err != nil {
		return "", false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	var parsed struct {
		IsTor bool   `json:"IsTor"`
		IP    string `json:"IP"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&parsed); err != nil {
		return "", false, fmt.Errorf("decoding check.torproject.org response: %w", err)
	}
	if parsed.IP == "" {
		return "", false, fmt.Errorf("check.torproject.org response had no IP")
	}
	return parsed.IP, parsed.IsTor, nil
}
