package routing

import (
	"github.com/xtls/xray-core/common/net"
)

// Context is a feature to store connection information for routing.
//
// xray:api:stable
type Context interface {
	// GetInboundTag returns the tag of the inbound the connection was from.
	GetInboundTag() string

	// GetSourceIPs returns the source IPs bound to the connection.
	GetSourceIPs() []net.IP

	// GetSourcePort returns the source port of the connection.
	GetSourcePort() net.Port

	// GetTargetIPs returns the target IP of the connection or resolved IPs of target domain.
	GetTargetIPs() []net.IP

	// GetTargetPort returns the target port of the connection.
	GetTargetPort() net.Port

	// GetLocalIPs returns the local IPs bound to the connection.
	GetLocalIPs() []net.IP

	// GetLocalPort returns the local port of the connection.
	GetLocalPort() net.Port

	// GetTargetDomain returns the target domain of the connection, if exists.
	GetTargetDomain() string

	// GetNetwork returns the network type of the connection.
	GetNetwork() net.Network

	// GetProtocol returns the protocol from the connection content, if sniffed out.
	GetProtocol() string

	// GetUser returns the user email from the connection content, if exists.
	GetUser() string

	// GetVlessRoute returns the user-sent VLESS UUID's 7th<<8 | 8th bytes, if exists.
	GetVlessRoute() net.Port

	// GetAttributes returns extra attributes from the conneciont content.
	GetAttributes() map[string]string

	// GetSkipDNSResolve returns a flag switch for weather skip dns resolve during route pick.
	GetSkipDNSResolve() bool
}
