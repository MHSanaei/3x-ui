// Package amneziawg manages native AmneziaWG interfaces (via awg-quick/awg,
// the AmneziaWG DKMS kernel module's userspace tools) as sidecars to the
// panel, the same way internal/mtproto manages mtg processes: one inbound
// row maps to one desired Instance, and a Manager reconciles the running
// interfaces toward whatever the database currently wants.
package amneziawg

import "github.com/mhsanaei/3x-ui/v3/internal/database/model"

// Obfuscation20 is an AmneziaWG 2.0 obfuscation parameter set (junk packets,
// padding, magic headers, the I1 signature packet). The same values must be
// applied on both ends of a tunnel, so the server stores them and every
// client config inherits them verbatim.
type Obfuscation20 struct {
	Jc   int    `json:"jc"`
	Jmin int    `json:"jmin"`
	Jmax int    `json:"jmax"`
	S1   int    `json:"s1"`
	S2   int    `json:"s2"`
	S3   int    `json:"s3"`
	S4   int    `json:"s4"`
	H1   string `json:"h1"`
	H2   string `json:"h2"`
	H3   string `json:"h3"`
	H4   string `json:"h4"`
	I1   string `json:"i1,omitempty"`
}

// Peer is one desired AmneziaWG peer: a client device the interface accepts.
// Email attributes traffic and online status back to the owning client, the
// same role SecretEntry.Name plays for mtproto.
type Peer struct {
	Email        string
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string

	// ForwardedPorts is a raw, user-supplied port list ("80, 443, 8000-8100")
	// DNAT'd to this peer's tunnel address. Empty means no port-forwarding.
	ForwardedPorts string

	// RouteThroughXray, when true, TPROXYs this peer's traffic (matched by its
	// tunnel source IP) into the single shared loopback Xray dokodemo-door
	// bridge (see amneziawgEgressPort in internal/web/service/xray.go) instead
	// of letting it NAT straight out through ExternalInterface. All routed
	// peers, across every AmneziaWG instance, share that one bridge and one
	// fwmark/policy-route pair; the per-peer distinction happens downstream in
	// Xray's own router, which the web service feeds a source-IP-matched rule
	// per peer. RouteOutboundTag is the Xray outbound/balancer tag that rule
	// targets; empty means Xray's default routing decides.
	RouteThroughXray bool
	RouteOutboundTag string
}

// Instance is the desired runtime configuration of one AmneziaWG inbound: a
// single interface (e.g. awg1) with a set of peers, mirroring how one mtproto
// inbound maps to one mtg process (internal/mtproto.Instance).
type Instance struct {
	Id            int
	Tag           string
	InterfaceName string
	ListenPort    int
	PrivateKey    string
	PublicKey     string
	// Address holds the interface's own tunnel address(es), e.g. "10.8.1.1/24".
	// Carries both the IPv4 and (when enabled) IPv6 server address.
	Address []string
	MTU     int

	Obfuscation Obfuscation20
	Peers       []Peer

	// ExternalInterface is the host NIC PostUp/PostDown NAT rules attach to.
	// Empty means auto-detect at config-generation time.
	ExternalInterface string

	// IPv6Enabled turns on the per-peer NDP proxy PostUp/PostDown entries
	// (ip -6 neigh add/del proxy) for peers that have an IPv6 AllowedIPs
	// entry. IPv6ExternalInterface overrides ExternalInterface for those
	// entries specifically; empty means reuse ExternalInterface.
	IPv6Enabled           bool
	IPv6ExternalInterface string
}

// ServerSettings is the "server" block of an AmneziaWG inbound's Settings
// JSON: the interface-level configuration shared by every client/peer. The
// listen port is deliberately not duplicated here — it lives on the inbound
// row itself (Inbound.Port), like every other protocol.
type ServerSettings struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`

	SubnetIP   string `json:"subnetIp"`
	SubnetCIDR int    `json:"subnetCidr"`
	MTU        int    `json:"mtu,omitempty"`

	// PrimaryDNS/SecondaryDNS seed the DNS line of downloadable client
	// configs; the server's own interface never sets one (see BuildClientConfig).
	PrimaryDNS   string `json:"primaryDns,omitempty"`
	SecondaryDNS string `json:"secondaryDns,omitempty"`

	// ExternalInterface is the host NIC PostUp/PostDown NAT rules attach to.
	// Empty means auto-detect.
	ExternalInterface string `json:"externalInterface,omitempty"`

	// IPv6Enabled turns on native IPv6 for clients: an IPv6 host address is
	// allocated from IPv6Subnet alongside each client's IPv4 one, and the
	// server proxies NDP for each enabled client's address so upstream
	// routers see it as directly reachable (no NAT66). IPv6ExternalInterface
	// overrides ExternalInterface for the NDP-proxy PostUp/PostDown entries
	// specifically; empty reuses ExternalInterface.
	IPv6Enabled           bool   `json:"ipv6Enabled,omitempty"`
	IPv6Subnet            string `json:"ipv6Subnet,omitempty"`
	IPv6ExternalInterface string `json:"ipv6ExternalInterface,omitempty"`

	// Obfuscation20's fields, repeated flat (not embedded) rather than
	// nested under their own key: encoding/json would happily inline an
	// embedded Obfuscation20 the same way, but the frontend's Go->Zod/TS
	// generator (tools/openapigen) does not — it emits a genuinely nested
	// `obfuscation20` object, which would silently diverge from the real
	// wire JSON. See Obfuscation() below for the manager-facing conversion.
	Jc   int    `json:"jc"`
	Jmin int    `json:"jmin"`
	Jmax int    `json:"jmax"`
	S1   int    `json:"s1"`
	S2   int    `json:"s2"`
	S3   int    `json:"s3"`
	S4   int    `json:"s4"`
	H1   string `json:"h1"`
	H2   string `json:"h2"`
	H3   string `json:"h3"`
	H4   string `json:"h4"`
	I1   string `json:"i1,omitempty"`
}

// Obfuscation extracts the Obfuscation20 parameter set from a ServerSettings
// block, for callers (the Manager, ValidateObfuscation) that want the
// grouped type rather than the flat wire fields.
func (s ServerSettings) Obfuscation() Obfuscation20 {
	return Obfuscation20{
		Jc: s.Jc, Jmin: s.Jmin, Jmax: s.Jmax,
		S1: s.S1, S2: s.S2, S3: s.S3, S4: s.S4,
		H1: s.H1, H2: s.H2, H3: s.H3, H4: s.H4,
		I1: s.I1,
	}
}

// InboundSettings is the full Settings JSON shape stored on an AmneziaWG
// inbound row: one server block plus the usual generic client list, so bulk
// operations, the QR modal and subscriptions all come from the same shared
// infrastructure every other protocol uses.
type InboundSettings struct {
	Server  *ServerSettings `json:"server"`
	Clients []model.Client  `json:"clients"`
}
