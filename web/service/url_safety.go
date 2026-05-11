package service

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// SanitizeHTTPURL validates and normalizes an http(s) URL without resolving
// DNS. Use SanitizePublicHTTPURL at the point of an outbound request.
func SanitizeHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("URL host is required")
	}
	clean := &url.URL{
		Scheme:   u.Scheme,
		Host:     u.Host,
		Path:     u.Path,
		RawPath:  u.RawPath,
		RawQuery: u.RawQuery,
		Fragment: u.Fragment,
	}
	return clean.String(), nil
}

// SanitizePublicHTTPURL validates and normalizes an http(s) URL, then blocks
// private/internal targets unless the caller explicitly allows them.
func SanitizePublicHTTPURL(raw string, allowPrivate bool) (string, error) {
	clean, err := SanitizeHTTPURL(raw)
	if err != nil || clean == "" {
		return clean, err
	}
	if allowPrivate {
		return clean, nil
	}
	u, err := url.Parse(clean)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rejectPrivateHost(ctx, u.Hostname()); err != nil {
		return "", err
	}
	return clean, nil
}

func rejectPrivateHost(ctx context.Context, hostname string) error {
	if ip := net.ParseIP(hostname); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked private/internal address %s", ip.String())
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		return fmt.Errorf("cannot resolve host %s: %w", hostname, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host %s has no IP addresses", hostname)
	}
	for _, ipAddr := range ips {
		if isBlockedIP(ipAddr.IP) {
			return fmt.Errorf("host %s resolves to blocked private/internal address %s", hostname, ipAddr.IP.String())
		}
	}
	return nil
}
