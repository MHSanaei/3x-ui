// Package amneziawg manages native AmneziaWG interfaces (via awg-quick/awg,
// the AmneziaWG DKMS kernel module's userspace tools) as sidecars to the
// panel, the same way internal/mtproto manages mtg processes: one inbound
// row maps to one desired Instance, and a Manager reconciles the running
// interfaces toward whatever the database currently wants.
package amneziawg

import "github.com/mhsanaei/3x-ui/v3/internal/database/model"

// Obfuscation31 is an AmneziaWG 3.1 obfuscation parameter set (junk packets,
// padding, magic headers, signature packets, header protection, timing
// randomization). The same values are applied on both ends of a tunnel, so
// the server stores them and every client config inherits them verbatim.
type Obfuscation31 struct {
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
	I2   string `json:"i2,omitempty"`
	I3   string `json:"i3,omitempty"`
	I4   string `json:"i4,omitempty"`
	I5   string `json:"i5,omitempty"`

	// HeaderProtectionKey is a base64 32-byte key shared by both ends (3.0
	// header protection); the ranges/booleans below are 3.x-only and optional.
	HeaderProtectionKey    string `json:"headerProtectionKey,omitempty"`
	ContentPaddingAddition string `json:"contentPaddingAddition,omitempty"`
	RekeyAfterTime         string `json:"rekeyAfterTime,omitempty"`
	RekeyTimeout           string `json:"rekeyTimeout,omitempty"`
	RejectAfterTime        string `json:"rejectAfterTime,omitempty"`
	KeepaliveTimeout       string `json:"keepaliveTimeout,omitempty"`
	MaxHandshakeAttempts   string `json:"maxHandshakeAttempts,omitempty"`
	RandomTrailers         bool   `json:"randomTrailers,omitempty"`
	DisableCookies         bool   `json:"disableCookies,omitempty"`
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

	Obfuscation Obfuscation31
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

	// RouteThroughXray gates the entire TPROXY-into-Xray bridge (see
	// EgressPortForInbound / injectAmneziawgEgress) for this instance: off by
	// default, so a plain AmneziaWG tunnel never depends on Xray being up at
	// all. Turning it on makes every peer's traffic TPROXY'd into this
	// instance's own loopback Xray bridge, tagged with the inbound's own
	// tag; the actual routing decision from there is left entirely to the
	// panel's stock Routing page (pick this inbound's tag as source, an
	// outbound, and optionally a peer's IP), exactly like routing any other
	// protocol -- only whether the bridge exists at all is a per-inbound
	// choice.
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

	// RouteThroughXray turns on this inbound's TPROXY-into-Xray bridge; see
	// Instance.RouteThroughXray for what that means. Off by default.
	RouteThroughXray bool `json:"routeThroughXray,omitempty"`

	// Obfuscation31's fields, repeated flat (not embedded) rather than
	// nested under their own key: encoding/json would happily inline an
	// embedded Obfuscation31 the same way, but the frontend's Go->Zod/TS
	// generator (tools/openapigen) does not — it emits a genuinely nested
	// `obfuscation31` object, which would silently diverge from the real
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
	I2   string `json:"i2,omitempty"`
	I3   string `json:"i3,omitempty"`
	I4   string `json:"i4,omitempty"`
	I5   string `json:"i5,omitempty"`

	HeaderProtectionKey    string `json:"headerProtectionKey,omitempty"`
	ContentPaddingAddition string `json:"contentPaddingAddition,omitempty"`
	RekeyAfterTime         string `json:"rekeyAfterTime,omitempty"`
	RekeyTimeout           string `json:"rekeyTimeout,omitempty"`
	RejectAfterTime        string `json:"rejectAfterTime,omitempty"`
	KeepaliveTimeout       string `json:"keepaliveTimeout,omitempty"`
	MaxHandshakeAttempts   string `json:"maxHandshakeAttempts,omitempty"`
	RandomTrailers         bool   `json:"randomTrailers,omitempty"`
	DisableCookies         bool   `json:"disableCookies,omitempty"`
}

// Obfuscation extracts the Obfuscation31 parameter set from a ServerSettings
// block, for callers (the Manager, ValidateObfuscation) that want the
// grouped type rather than the flat wire fields.
func (s ServerSettings) Obfuscation() Obfuscation31 {
	return Obfuscation31{
		Jc: s.Jc, Jmin: s.Jmin, Jmax: s.Jmax,
		S1: s.S1, S2: s.S2, S3: s.S3, S4: s.S4,
		H1: s.H1, H2: s.H2, H3: s.H3, H4: s.H4,
		I1: s.I1, I2: s.I2, I3: s.I3, I4: s.I4, I5: s.I5,
		HeaderProtectionKey:    s.HeaderProtectionKey,
		ContentPaddingAddition: s.ContentPaddingAddition,
		RekeyAfterTime:         s.RekeyAfterTime,
		RekeyTimeout:           s.RekeyTimeout,
		RejectAfterTime:        s.RejectAfterTime,
		KeepaliveTimeout:       s.KeepaliveTimeout,
		MaxHandshakeAttempts:   s.MaxHandshakeAttempts,
		RandomTrailers:         s.RandomTrailers,
		DisableCookies:         s.DisableCookies,
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
