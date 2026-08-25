package amneziawg

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// OutboundPeer describes one remote AmneziaWG server this outbound dials.
// PublicKey is the SERVER's public key; AllowedIPs selects which destinations
// ride the tunnel; Endpoint is the server's "host:port".
type OutboundPeer struct {
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string
	Endpoint     string
	KeepAlive    int
}

// OutboundInstance is the desired runtime configuration of one AmneziaWG
// outbound: a client-mode interface dialing the remote endpoint -- the mirror
// image of Instance (server mode), consumed by internal/amneziawgnet's
// outbound Manager.
type OutboundInstance struct {
	Tag         string
	Address     []string
	MTU         int
	PrivateKey  string
	Obfuscation Obfuscation31
	Peers       []OutboundPeer
	ListenPort  int
}

// OutboundSettings is the Settings JSON stored on an "amneziawg" outbound row
// in the Xray template. Field names match ServerSettings' flat obfuscation
// keys so one parameter set pastes cleanly between an inbound's server block
// and an outbound's settings (both ends of a tunnel must carry identical
// values). tools/openapigen is not involved: this shape never crosses the
// OpenAPI boundary, it travels inside the template JSON string.
type OutboundSettings struct {
	MTU        int      `json:"mtu,omitempty"`
	SecretKey  string   `json:"secretKey"`
	Address    []string `json:"address"`
	ListenPort int      `json:"listenPort,omitempty"`

	// Flat Obfuscation31 mirror -- see OutboundSettings' doc comment.
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
	RandomTrailers         bool   `json:"randomTrailers"`
	DisableCookies         bool   `json:"disableCookies"`

	Peers []OutboundSettingsPeer `json:"peers"`
}

// OutboundSettingsPeer is one entry of OutboundSettings.Peers.
type OutboundSettingsPeer struct {
	PublicKey    string   `json:"publicKey"`
	PresharedKey string   `json:"presharedKey,omitempty"`
	AllowedIPs   []string `json:"allowedIPs"`
	Endpoint     string   `json:"endpoint"`
	KeepAlive    int      `json:"keepAlive,omitempty"`
}

// Obfuscation folds the flat wire fields back into the grouped type, matching
// ServerSettings.Obfuscation.
func (s OutboundSettings) Obfuscation() Obfuscation31 {
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

// IsAmneziaWGOutbound reports whether a raw outbound JSON object from the
// Xray template carries the panel's amneziawg pseudo-protocol.
func IsAmneziaWGOutbound(raw []byte) bool {
	var probe struct {
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.Protocol == "amneziawg"
}

// outboundSettingsOf extracts the nested "settings" block from a raw
// amneziawg template outbound.
func outboundSettingsOf(raw []byte) (json.RawMessage, bool) {
	var wrapper struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil || len(wrapper.Settings) == 0 {
		return nil, false
	}
	return wrapper.Settings, true
}

// InstanceFromOutbound derives a client-mode OutboundInstance from one raw
// template outbound (protocol must already be "amneziawg") and its tag.
// Returns false when settings are unusable: unparseable, no usable address,
// invalid keys, or no peer with a valid key + endpoint + at least one
// AllowedIPs entry.
func InstanceFromOutbound(tag string, raw []byte) (OutboundInstance, bool) {
	settingsRaw, ok := outboundSettingsOf(raw)
	if !ok {
		return OutboundInstance{}, false
	}
	var parsed OutboundSettings
	if err := json.Unmarshal(settingsRaw, &parsed); err != nil {
		return OutboundInstance{}, false
	}
	inst := OutboundInstance{
		Tag:        tag,
		Address:    parsed.Address,
		MTU:        parsed.MTU,
		PrivateKey: parsed.SecretKey,
		ListenPort: parsed.ListenPort,
		Obfuscation: Obfuscation31{
			Jc: parsed.Jc, Jmin: parsed.Jmin, Jmax: parsed.Jmax,
			S1: parsed.S1, S2: parsed.S2, S3: parsed.S3, S4: parsed.S4,
			H1: parsed.H1, H2: parsed.H2, H3: parsed.H3, H4: parsed.H4,
			I1: parsed.I1, I2: parsed.I2, I3: parsed.I3, I4: parsed.I4, I5: parsed.I5,
			HeaderProtectionKey:    parsed.HeaderProtectionKey,
			ContentPaddingAddition: parsed.ContentPaddingAddition,
			RekeyAfterTime:         parsed.RekeyAfterTime,
			RekeyTimeout:           parsed.RekeyTimeout,
			RejectAfterTime:        parsed.RejectAfterTime,
			KeepaliveTimeout:       parsed.KeepaliveTimeout,
			MaxHandshakeAttempts:   parsed.MaxHandshakeAttempts,
			RandomTrailers:         parsed.RandomTrailers,
			DisableCookies:         parsed.DisableCookies,
		},
	}
	for _, p := range parsed.Peers {
		if p.PublicKey == "" || len(p.AllowedIPs) == 0 || p.Endpoint == "" {
			continue
		}
		inst.Peers = append(inst.Peers, OutboundPeer(p))
	}
	if len(inst.Address) == 0 || len(inst.Peers) == 0 {
		return OutboundInstance{}, false
	}
	return inst, true
}

// validateEndpoint accepts "host:port" (or "[v6]:port") with a numeric port
// in range and a host free of control characters -- amneziawg-go resolves the
// host itself, so no IP-format restriction here beyond sanity.
func validateEndpoint(ep string) error {
	if ep == "" {
		return fmt.Errorf("endpoint is required")
	}
	if err := ValidateConfigValue("endpoint", ep); err != nil {
		return err
	}
	host, portS, err := net.SplitHostPort(ep)
	if err != nil {
		return fmt.Errorf("invalid endpoint %q: must be host:port", ep)
	}
	port, err := strconv.Atoi(portS)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("invalid endpoint %q: bad port", ep)
	}
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("invalid endpoint %q: empty host", ep)
	}
	return nil
}

// validateTunnelAddresses requires every entry to be a parseable IP prefix
// (the outbound's own tunnel address(es), e.g. "10.8.1.2/32").
func validateTunnelAddresses(addrs []string) error {
	if len(addrs) == 0 {
		return fmt.Errorf("at least one tunnel address is required")
	}
	for _, a := range addrs {
		prefix, err := netip.ParsePrefix(a)
		if err != nil {
			return fmt.Errorf("invalid tunnel address %q: %w", a, err)
		}
		_ = prefix
	}
	return nil
}

// ValidateAmneziaWGOutbound checks one raw template outbound's settings the
// way normalizeAmneziaWGSettings checks an inbound: enough that a bad entry
// can neither break the embedded device's UAPI apply nor smuggle control
// characters into anything rendered downstream.
func ValidateAmneziaWGOutbound(tag string, raw []byte) error {
	settingsRaw, ok := outboundSettingsOf(raw)
	if !ok {
		return fmt.Errorf("amneziawg outbound %q: missing settings block", tag)
	}
	var parsed OutboundSettings
	if err := json.Unmarshal(settingsRaw, &parsed); err != nil {
		return fmt.Errorf("amneziawg outbound %q: invalid settings: %w", tag, err)
	}
	if err := validateTunnelAddresses(parsed.Address); err != nil {
		return fmt.Errorf("amneziawg outbound %q: %w", tag, err)
	}
	if _, err := wireguard.KeyToHex(parsed.SecretKey); err != nil {
		return fmt.Errorf("amneziawg outbound %q: invalid privateKey: %w", tag, err)
	}
	if err := ValidateObfuscation(parsed.Obfuscation()); err != nil {
		return fmt.Errorf("amneziawg outbound %q: %w", tag, err)
	}
	if err := validateHeaderProtectionKey(parsed.HeaderProtectionKey); err != nil {
		return fmt.Errorf("amneziawg outbound %q: %w", tag, err)
	}
	if len(parsed.Peers) == 0 {
		return fmt.Errorf("amneziawg outbound %q: at least one peer is required", tag)
	}
	for i, p := range parsed.Peers {
		if _, err := wireguard.KeyToHex(p.PublicKey); err != nil {
			return fmt.Errorf("amneziawg outbound %q: peer %d: invalid publicKey: %w", tag, i, err)
		}
		if p.PresharedKey != "" {
			if _, err := wireguard.KeyToHex(p.PresharedKey); err != nil {
				return fmt.Errorf("amneziawg outbound %q: peer %d: invalid presharedKey: %w", tag, i, err)
			}
		}
		if err := validateEndpoint(p.Endpoint); err != nil {
			return fmt.Errorf("amneziawg outbound %q: peer %d: %w", tag, i, err)
		}
		if len(p.AllowedIPs) == 0 {
			return fmt.Errorf("amneziawg outbound %q: peer %d: at least one allowedIPs entry is required", tag, i)
		}
		for _, a := range p.AllowedIPs {
			if _, err := netip.ParsePrefix(strings.TrimSpace(a)); err != nil {
				return fmt.Errorf("amneziawg outbound %q: peer %d: invalid allowedIP %q: %w", tag, i, a, err)
			}
		}
	}
	return nil
}
