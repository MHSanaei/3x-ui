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

	Obfuscation Obfuscation20

	// HeaderProtectionKey and ContentPaddingAddition are AmneziaWG 3.0's
	// device-wide obfuscation fields. Deliberately not part of
	// Obfuscation20 (2.0-only, see its own doc comment) --
	// InstanceFromInbound copies them from ServerSettings' identically
	// named flat fields below, and amneziawgnet.DeviceOptions is what
	// actually consumes them (see that type's own doc comment for why
	// they live outside this shared schema). HeaderProtectionKey empty
	// (the default) disables AWG 3.0 header protection entirely; unlike
	// every other obfuscation field here, it is never auto-generated --
	// turning it on is a compatibility-breaking, opt-in decision an admin
	// must make explicitly (see GenerateObfuscation20's own doc comment).
	HeaderProtectionKey    string
	ContentPaddingAddition string

	// RekeyAfterTime, RekeyTimeout, RejectAfterTime, KeepaliveTimeout, and
	// MaxHandshakeAttempts are AmneziaWG 3.0's five device-wide session-
	// timing fields -- each a "low-high" range or bare integer (identical
	// grammar/width to H1-H4 and ContentPaddingAddition above; confirmed
	// against amneziawg-go v3.0.3's device/uapi.go, which parses all of
	// these through the same UintRange.FromString), left empty (the
	// default) to keep amneziawg-go's own real-protocol defaults: 120s,
	// 5s, 180s, 10s, and 18 attempts respectively (device/constants.go).
	// Like HeaderProtectionKey, these are opt-in tuning knobs, never
	// auto-generated as part of the classic obfuscation set.
	RekeyAfterTime       string
	RekeyTimeout         string
	RejectAfterTime      string
	KeepaliveTimeout     string
	MaxHandshakeAttempts string

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

	// HeaderProtectionKey and ContentPaddingAddition are AmneziaWG 3.0
	// fields, flat and top-level for the same tools/openapigen reason as
	// the block above -- deliberately NOT part of Obfuscation20/
	// Obfuscation() below, matching Instance's own separation.
	// HeaderProtectionKey is a base64 32-byte key; empty (the default)
	// disables AWG 3.0 header protection. A non-empty value requires
	// every one of S1-S4 above to be >= 12 -- ValidateHeaderProtection
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

	// AwgVersion is the admin-declared AmneziaWG protocol-version ceiling
	// for this inbound: AwgVersion2 (default) or AwgVersion3. Purely a
	// save-time/UI gate -- amneziawgnet's actual UAPI config never reads
	// it, only HeaderProtectionKey/ContentPaddingAddition directly -- it
	// exists so enabling either of those two fields is a deliberate,
	// visible choice instead of something that silently starts working
	// (or silently keeps working after being turned back off) with no
	// admin-facing signal at all. See EffectiveAwgVersion/ValidateAwgVersion
	// in params.go for the back-compat and consistency rules this field
	// follows.
	AwgVersion string `json:"awgVersion,omitempty"`
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
