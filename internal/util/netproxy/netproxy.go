// Package netproxy builds HTTP clients that route the panel's own outbound
// requests through an admin-configured proxy, used to reach GitHub and Telegram
// from servers where those services are filtered.
package netproxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// NewHTTPClient returns an *http.Client whose transport honors proxyURL.
//
// An empty proxyURL yields a plain client (unchanged behavior). socks5/socks5h
// URLs are dialed through golang.org/x/net/proxy; http/https URLs use the
// standard library proxy support. Any other scheme or URL without a host
// returns an error so callers can log it and fall back to a direct connection.
//
// The proxy address is intentionally not subjected to SSRF filtering: it is
// admin-configured and is commonly a loopback/private address (for example a
// local Xray SOCKS inbound).
func NewHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return &http.Client{Timeout: timeout}, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parse proxy url: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "socks5", "socks5h", "http", "https":
		if parsed.Host == "" {
			return nil, fmt.Errorf("proxy url %q has no host", proxyURL)
		}
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q", parsed.Scheme)
	}

	transport := baseTransport()

	switch scheme {
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsed.User != nil {
			password, _ := parsed.User.Password()
			auth = &proxy.Auth{User: parsed.User.Username(), Password: password}
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("create socks5 dialer: %w", err)
		}
		if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsed)
	}

	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

func baseTransport() *http.Transport {
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		return base.Clone()
	}
	return &http.Transport{}
}
