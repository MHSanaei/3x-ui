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

	// ExternalInterface named the host NIC PostUp/PostDown NAT rules
	// attached to under the retired kernel-module architecture. Not read by
	// the embedded path (internal/amneziawgnet) as of the hard cutover --
	// kept for Phase 3.5's planned real-IPv6-address-alias mechanism, which
	// will need to know which host NIC to alias an address onto.
	ExternalInterface string

	// IPv6Enabled/IPv6ExternalInterface controlled the per-peer NDP proxy
	// PostUp/PostDown entries (ip -6 neigh add/del proxy) under the retired
	// kernel-module architecture. Not read by the embedded path as of the
	// hard cutover -- distinct-per-peer public IPv6 identity is Phase 3.5,
	// see the migration plan.
	IPv6Enabled           bool
	IPv6ExternalInterface string

	// RouteThroughXray gated the kernel-module architecture's opt-in
	// TPROXY-into-Xray bridge. The embedded path (internal/amneziawgnet)
	// has no equivalent opt-in at all -- every peer's traffic already goes
	// through Xray's own SOCKS5 inbound unconditionally, since there's no
	// other way for decapsulated gVisor traffic to reach the real internet
	// -- so this field is now vestigial: read from existing stored settings
	// for backward compatibility, but not acted on by anything. Slated for
	// removal alongside the frontend toggle in a follow-up.
	RouteThroughXray bool
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

	// ExternalInterface, IPv6Enabled/IPv6Subnet/IPv6ExternalInterface, and
	// RouteThroughXray are all vestigial as of the hard cutover to the
	// embedded path (internal/amneziawgnet) -- see the matching fields on
	// Instance for what each used to do under the retired kernel-module
	// architecture and what (if anything) is planned to read them again.
	ExternalInterface string `json:"externalInterface,omitempty"`

	IPv6Enabled           bool   `json:"ipv6Enabled,omitempty"`
	IPv6Subnet            string `json:"ipv6Subnet,omitempty"`
	IPv6ExternalInterface string `json:"ipv6ExternalInterface,omitempty"`

	RouteThroughXray bool `json:"routeThroughXray,omitempty"`

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
