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
	Address []string
	MTU     int

	Obfuscation Obfuscation20
	Peers       []Peer

	// ExternalInterface is the host NIC PostUp/PostDown NAT rules attach to.
	// Empty means auto-detect at config-generation time.
	ExternalInterface string
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

	// Obfuscation20 is embedded (not nested) so its fields (jc, jmin, s1...)
	// sit flat in the JSON alongside the rest of the server block, matching
	// the upstream AmneziaWG PR's schema.
	Obfuscation20
}

// InboundSettings is the full Settings JSON shape stored on an AmneziaWG
// inbound row: one server block plus the usual generic client list, so bulk
// operations, the QR modal and subscriptions all come from the same shared
// infrastructure every other protocol uses.
type InboundSettings struct {
	Server  *ServerSettings `json:"server"`
	Clients []model.Client  `json:"clients"`
}
