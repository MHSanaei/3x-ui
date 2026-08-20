// Copyright (c) 2026 Masterain. MIT License.
// Adapted from PIA-Wireguard-Config-Generator-GUI (commit 53686fcd).
package pia

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"
)

type RegistrationClient struct {
	CAPEM     []byte
	Port      uint16
	MaxBody   int64
	Timeout   time.Duration
	UserAgent string
}

func NewRegistrationClient(caPEM []byte) *RegistrationClient {
	return &RegistrationClient{
		CAPEM: caPEM, Port: DefaultAddKeyPort, MaxBody: DefaultMaxResponseBody,
		Timeout: DefaultRequestTimeout, UserAgent: DefaultUserAgent,
	}
}

func (c *RegistrationClient) RegisterKey(ctx context.Context, server WireGuardServer, token string, publicKey string) (Registration, error) {
	if !server.IP.IsValid() || !server.IP.Is4() || !validHostname(server.Hostname) {
		return Registration{}, NewError(CodeInvalidInput, "The selected PIA WireGuard server is invalid.")
	}
	if !validSecret([]byte(token), 16, 4096) {
		return Registration{}, NewError(CodeTokenRejected, "The PIA authentication token is invalid.")
	}
	if !validWGKey(publicKey) {
		return Registration{}, NewError(CodeInvalidInput, "The WireGuard public key is invalid.")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(c.CAPEM) {
		return Registration{}, NewError(CodeTLSValidation, "The built-in PIA certificate authority is invalid.")
	}
	port := c.Port
	if port == 0 {
		port = DefaultAddKeyPort
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, net.JoinHostPort(server.IP.String(), strconv.Itoa(int(port))))
		},
		TLSClientConfig:     &tls.Config{ServerName: server.Hostname, RootCAs: roots, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: 8 * time.Second, ResponseHeaderTimeout: 12 * time.Second, ForceAttemptHTTP2: true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: c.Timeout, CheckRedirect: noRedirect}
	endpoint := url.URL{Scheme: "https", Host: net.JoinHostPort(server.Hostname, strconv.Itoa(int(port))), Path: "/addKey"}
	query := endpoint.Query()
	query.Set("pt", token)
	query.Set("pubkey", publicKey)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Registration{}, WrapError(CodeRegistrationRejected, "Could not prepare PIA key registration.", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.UserAgent)
	response, err := client.Do(request)
	if err != nil {
		return Registration{}, classifyNetworkError(ctx, CodeNetworkUnavailable, "The selected PIA WireGuard server could not be reached.", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return Registration{}, NewError(CodeTokenRejected, "The PIA authentication token was rejected.")
	}
	if response.StatusCode != http.StatusOK {
		return Registration{}, NewError(CodeRegistrationRejected, fmt.Sprintf("PIA key registration returned HTTP %d.", response.StatusCode))
	}
	if !expectedContentType(response.Header.Get("Content-Type"), "application/json") {
		return Registration{}, NewError(CodeRegistrationInvalid, "PIA key registration returned an unexpected content type.")
	}
	raw, err := readLimitedBody(response.Body, c.MaxBody)
	if err != nil {
		return Registration{}, WrapError(CodeRegistrationInvalid, "PIA key registration returned an invalid response.", err)
	}
	return parseRegistration(raw)
}

func parseRegistration(raw []byte) (Registration, error) {
	var payload struct {
		Status     string   `json:"status"`
		PeerIP     string   `json:"peer_ip"`
		ServerKey  string   `json:"server_key"`
		ServerIP   string   `json:"server_ip"`
		ServerPort int      `json:"server_port"`
		DNSServers []string `json:"dns_servers"`
	}
	if err := decodeSingleJSON(raw, &payload); err != nil {
		return Registration{}, NewError(CodeRegistrationInvalid, "PIA key registration returned malformed JSON.")
	}
	if payload.Status != "OK" {
		return Registration{}, NewError(CodeRegistrationRejected, "The PIA server rejected WireGuard key registration.")
	}
	peerIP, err := parsePeerIP(payload.PeerIP)
	if err != nil {
		return Registration{}, NewError(CodeRegistrationInvalid, "PIA returned an invalid WireGuard peer address.")
	}
	if !validWGKey(payload.ServerKey) {
		return Registration{}, NewError(CodeRegistrationInvalid, "PIA returned an invalid WireGuard server key.")
	}
	serverIP, err := netip.ParseAddr(payload.ServerIP)
	if err != nil || !serverIP.Is4() || serverIP.IsUnspecified() {
		return Registration{}, NewError(CodeRegistrationInvalid, "PIA returned an invalid WireGuard server address.")
	}
	if payload.ServerPort < 1 || payload.ServerPort > 65535 {
		return Registration{}, NewError(CodeRegistrationInvalid, "PIA returned an invalid WireGuard server port.")
	}
	if len(payload.DNSServers) == 0 || len(payload.DNSServers) > 8 {
		return Registration{}, NewError(CodeRegistrationInvalid, "PIA returned no valid DNS servers.")
	}
	dns := make([]netip.Addr, 0, len(payload.DNSServers))
	for _, value := range payload.DNSServers {
		address, err := netip.ParseAddr(value)
		if err != nil || !address.Is4() || address.IsUnspecified() {
			return Registration{}, NewError(CodeRegistrationInvalid, "PIA returned an invalid DNS server.")
		}
		dns = append(dns, address)
	}
	return Registration{PeerIP: peerIP, ServerKey: payload.ServerKey, ServerIP: serverIP, ServerPort: uint16(payload.ServerPort), DNSServers: dns}, nil
}

func parsePeerIP(value string) (netip.Prefix, error) {
	if address, err := netip.ParseAddr(value); err == nil {
		if !address.Is4() || address.IsUnspecified() {
			return netip.Prefix{}, fmt.Errorf("peer address is not a usable IPv4 address")
		}
		return netip.PrefixFrom(address, 32), nil
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.Addr().Is4() || prefix.Addr().IsUnspecified() || prefix.Bits() != 32 {
		return netip.Prefix{}, fmt.Errorf("peer address is not an IPv4 host prefix")
	}
	return prefix, nil
}
