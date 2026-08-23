// Package pia is a standalone PIA WireGuard control-plane client.
package pia

import (
	"context"
	"net/netip"
	"time"
)

type Region struct {
	ID             string
	Name           string
	CountryCode    string
	Geo            bool
	PortForwarding bool
	WireGuard      []WireGuardServer
}

type WireGuardServer struct {
	Hostname string
	IP       netip.Addr
}

type Token struct {
	Value     []byte
	ExpiresAt time.Time
}

type Registration struct {
	PeerIP     netip.Prefix
	ServerKey  string
	ServerIP   netip.Addr
	ServerPort uint16
	DNSServers []netip.Addr
}

type Authenticator interface {
	Authenticate(ctx context.Context, username string, password []byte) (Token, error)
}

type Registrar interface {
	RegisterKey(ctx context.Context, server WireGuardServer, token string, publicKey string) (Registration, error)
}
