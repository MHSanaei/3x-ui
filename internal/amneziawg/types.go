// Package amneziawg holds the AmneziaWG protocol's shared, DB-backed shapes
// (Instance, Peer, Obfuscation31, ServerSettings/InboundSettings) and the
// pure functions that derive an Instance from a stored inbound row. It has
// no OS dependency of its own: internal/amneziawgnet embeds amneziawg-go
// over a gVisor netstack and owns the actual running interfaces, one
// Manager-managed Device per desired Instance -- see that package's Manager
// for the reconcile-on-tick lifecycle (modeled on internal/mtproto's own
// Manager), and instance.go's own doc comment for how this package's role
// narrowed to protocol-shape-only after the kernel-module (DKMS) + awg-quick
// architecture this fork originally shipped was retired.
package amneziawg

import "github.com/mhsanaei/3x-ui/v3/internal/database/model"

// Obfuscation31 is an AmneziaWG 3.1 obfuscation parameter set (junk packets,
// padding, magic headers, the five CPS signature-packet slots, and the 3.x
// header-protection/content-padding/timing/boolean fields). The same values
// must be applied on both ends of a tunnel, so the server stores them and
// every client config inherits them verbatim.
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
	// I1-I5 are the real protocol's five CPS signature-packet slots
	// (confirmed against amneziawg-go v3.0.3's device/uapi.go: "i1"
	// through "i5" are five independent UAPI setters, device.ipackets[0..4],
	// all parsed via the identical newObfChain grammar).
	I1 string `json:"i1,omitempty"`
	I2 string `json:"i2,omitempty"`
	I3 string `json:"i3,omitempty"`
	I4 string `json:"i4,omitempty"`
	I5 string `json:"i5,omitempty"`

	// HeaderProtectionKey is a base64 32-byte key shared by both ends; the
	// ranges/booleans below are 3.x-only and optional.
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
	// forwarded to this peer's tunnel address by internal/amneziawgnet's
	// PortForwardSet listener supervisor. Empty means no port-forwarding.
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

	// Obfuscation carries the full AmneziaWG 3.1 parameter set, including
	// the 3.x header-protection/content-padding/timing/boolean fields (see
	// Obfuscation31's own doc comment) -- amneziawgnet.DeviceOptions is
	// what actually consumes it when building the embedded Device's UAPI
	// config.
	Obfuscation Obfuscation31

	Peers []Peer

	// ExternalInterface named the host NIC PostUp/PostDown NAT rules
	// attached to under the retired kernel-module architecture. Also the
	// fallback host NIC internal/amneziawgnet's IPv6-address-alias
	// mechanism (desiredV6Aliases) uses when IPv6ExternalInterface is left
	// blank.
	ExternalInterface string

	// IPv6Enabled/IPv6ExternalInterface gate internal/amneziawgnet's
	// IPv6-address-alias mechanism (desiredV6Aliases,
	// internal/web/service/xray.go's injectAmneziawgV6Egress): each peer
	// with an IPv6 AllowedIPs entry gets that address aliased onto this
	// host NIC (ip -6 addr add) and a dedicated Xray freedom outbound bound
	// to it, giving that peer's own outbound connections a distinct public
	// source identity. Narrower in scope than these identically-named
	// fields' role under the retired kernel-module architecture, which used
	// per-peer NDP-proxy entries (ip -6 neigh add proxy) to also support
	// unsolicited inbound connections toward the peer -- that capability is
	// the separate, not-yet-built Phase 3.6 (port-forwarding).
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

	// ExternalInterface, IPv6Enabled, and IPv6ExternalInterface are live
	// again as of Phase 3.5 -- see the matching fields on Instance for what
	// they gate (internal/amneziawgnet's IPv6-address-alias mechanism).
	// IPv6Subnet was never actually vestigial either: InstanceFromInbound
	// already consumes it (via serverAddressV6) to build the server's own
	// tunnel address, same as always. Only RouteThroughXray, below, remains
	// genuinely vestigial as of the hard cutover to the embedded path
	// (internal/amneziawgnet) -- read from existing stored settings for
	// backward compatibility, but not acted on by anything.
	ExternalInterface string `json:"externalInterface,omitempty"`

	IPv6Enabled           bool   `json:"ipv6Enabled,omitempty"`
	IPv6Subnet            string `json:"ipv6Subnet,omitempty"`
	IPv6ExternalInterface string `json:"ipv6ExternalInterface,omitempty"`

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

	// HeaderProtectionKey and ContentPaddingAddition are AmneziaWG 3.0
	// fields, flat and top-level for the same tools/openapigen reason as
	// the block above; Obfuscation() below folds them back into
	// Obfuscation31's own identically named fields.
	// HeaderProtectionKey is a base64 32-byte key; empty (the default)
	// disables AWG 3.0 header protection. A non-empty value requires
	// every one of S1-S4 above to be >= 12 -- ValidateObfuscation
	// enforces this at save time, not just at IpcSet time.
	// ContentPaddingAddition is a "low-high" range or bare integer, the
	// same grammar as H1-H4 but capped at uint16 max.
	HeaderProtectionKey    string `json:"headerProtectionKey,omitempty"`
	ContentPaddingAddition string `json:"contentPaddingAddition,omitempty"`

	// RekeyAfterTime/RekeyTimeout/RejectAfterTime/KeepaliveTimeout/
	// MaxHandshakeAttempts mirror Instance's identically named fields --
	// see that type's own doc comment for the grammar/width/real-default
	// details. Flat and top-level for the same tools/openapigen reason as
	// the rest of this struct.
	RekeyAfterTime       string `json:"rekeyAfterTime,omitempty"`
	RekeyTimeout         string `json:"rekeyTimeout,omitempty"`
	RejectAfterTime      string `json:"rejectAfterTime,omitempty"`
	KeepaliveTimeout     string `json:"keepaliveTimeout,omitempty"`
	MaxHandshakeAttempts string `json:"maxHandshakeAttempts,omitempty"`

	// RandomTrailers/DisableCookies mirror Instance's identically named
	// AmneziaWG 3.1 fields -- see that type's own doc comment for the real
	// protocol/interop details. Both real bool fields (not omitempty):
	// buildUAPIConfig always emits both lines explicitly so the
	// reconfigure-in-place diff correctly notices a true->false edit, not
	// just false->true.
	RandomTrailers bool `json:"randomTrailers"`
	DisableCookies bool `json:"disableCookies"`
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
