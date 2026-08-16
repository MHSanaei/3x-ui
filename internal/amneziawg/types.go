// Package amneziawg manages native AmneziaWG interfaces (via awg-quick/awg) as
// panel sidecars, the way internal/mtproto manages mtg: one inbound row maps to
// one Instance, and a Manager reconciles the running interfaces toward the DB.
package amneziawg

import "github.com/mhsanaei/3x-ui/v3/internal/database/model"

// Obfuscation31 is an AmneziaWG 3.1 obfuscation parameter set. Both ends of a
// tunnel must match, so every client config inherits the server's values.
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

// Peer is one desired AmneziaWG peer. Email attributes traffic and online
// status back to the owning client, as SecretEntry.Name does for mtproto.
type Peer struct {
	Email        string
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string

	// ForwardedPorts is a raw, user-supplied port list ("80, 443, 8000-8100")
	// DNAT'd to this peer's tunnel address. Empty means no port-forwarding.
	ForwardedPorts string
}

// Instance is one AmneziaWG inbound's desired runtime state: a single
// interface (e.g. awg1) with a set of peers.
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

	// IPv6Enabled turns on per-peer NDP proxy PostUp/PostDown entries.
	// IPv6ExternalInterface overrides ExternalInterface for those only.
	IPv6Enabled           bool
	IPv6ExternalInterface string

	// RouteThroughXray gates this instance's whole TPROXY-into-Xray bridge; off
	// by default, so a plain tunnel never depends on Xray running at all. Where
	// routed traffic then goes is left to the panel's stock Routing page.
	RouteThroughXray bool
}

// ServerSettings is the "server" block of an AmneziaWG inbound's Settings JSON.
// The listen port lives on the inbound row itself, like every other protocol.
type ServerSettings struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`

	SubnetIP   string `json:"subnetIp"`
	SubnetCIDR int    `json:"subnetCidr"`
	MTU        int    `json:"mtu,omitempty"`

	// PrimaryDNS/SecondaryDNS seed downloadable client configs only; the
	// server's own interface never sets a DNS line.
	PrimaryDNS   string `json:"primaryDns,omitempty"`
	SecondaryDNS string `json:"secondaryDns,omitempty"`

	// ExternalInterface is the host NIC PostUp/PostDown NAT rules attach to.
	// Empty means auto-detect.
	ExternalInterface string `json:"externalInterface,omitempty"`

	// IPv6Enabled allocates each client an IPv6 host address from IPv6Subnet and
	// proxies NDP for it, so upstream routers see it as directly reachable
	// (no NAT66). IPv6ExternalInterface overrides ExternalInterface for those.
	IPv6Enabled           bool   `json:"ipv6Enabled,omitempty"`
	IPv6Subnet            string `json:"ipv6Subnet,omitempty"`
	IPv6ExternalInterface string `json:"ipv6ExternalInterface,omitempty"`

	// RouteThroughXray turns on this inbound's TPROXY-into-Xray bridge; see
	// Instance.RouteThroughXray for what that means. Off by default.
	RouteThroughXray bool `json:"routeThroughXray,omitempty"`

	// Obfuscation31's fields repeated flat, not embedded: encoding/json would
	// inline an embedded struct but tools/openapigen emits a nested object,
	// which would silently diverge from the real wire JSON.
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

// Obfuscation regroups the flat wire fields above into Obfuscation31, for
// callers that want the grouped type.
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

// InboundSettings is an AmneziaWG inbound's full Settings JSON: one server
// block plus the generic client list every other protocol's tooling reads.
type InboundSettings struct {
	Server  *ServerSettings `json:"server"`
	Clients []model.Client  `json:"clients"`
}
