package amneziawgnet

import (
	"fmt"
	"net/netip"
	"strings"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// defaultMTU matches internal/amneziawg's own kernel-module interface
// default -- 1420, WireGuard/AmneziaWG's usual accounting for tunnel
// encapsulation overhead on a standard 1500-byte-MTU host link.
const defaultMTU = 1420

// DeviceOptions carries AmneziaWG 3.0's device-wide fields (header
// protection, content padding, and the five session-timing knobs) --
// mirrored from amneziawg.Instance's identically named fields by every
// caller (see the 3 Desired{} call sites), not read from Instance
// directly, since amneziawgnet has no dependency on internal/amneziawg
// beyond the plain data types it already imports. Zero-value DeviceOptions
// means amneziawg-go's own real-protocol defaults throughout: classic
// (non-3.0) obfuscation, and its built-in session timings (120s/5s/180s/
// 10s/18 attempts -- device/constants.go).
type DeviceOptions struct {
	// HeaderProtectionKey is a base64 32-byte key. Empty disables AWG 3.0
	// header protection entirely. Non-empty requires every one of
	// Obfuscation20.S1-S4 to be >= 12 (amneziawg-go's own HeaderCipherNonceSize
	// requirement) -- IpcSet will reject the config otherwise.
	HeaderProtectionKey string
	// ContentPaddingAddition, RekeyAfterTime, RekeyTimeout, RejectAfterTime,
	// KeepaliveTimeout, and MaxHandshakeAttempts are each a "low-high" range
	// (or a bare integer), amneziawg-go's own UintRange.FromString grammar
	// (confirmed directly against v3.0.3's device/uapi.go -- all six share
	// the identical parser). Empty leaves that one field at amneziawg-go's
	// own default.
	ContentPaddingAddition string
	RekeyAfterTime         string
	RekeyTimeout           string
	RejectAfterTime        string
	KeepaliveTimeout       string
	MaxHandshakeAttempts   string
	// Logger is passed to device.NewDevice as-is; nil uses a silent logger
	// (device.NewLogger(device.LogLevelSilent, "")).
	Logger *device.Logger
}

// Device is one running embedded AmneziaWG interface: an amneziawg-go
// Device over a gVisor netstack, plus the raw *stack.Stack a caller needs to
// attach a TCP/UDP forwarder (see forwarder.go / udp.go). Closing it tears
// down both the WireGuard device and the underlying tun/stack.
type Device struct {
	*device.Device
	Stack *stack.Stack
}

// NewDevice constructs, configures, and brings up an embedded AmneziaWG
// interface for inst in one call: a gVisor-backed tun.Device sized to
// inst.MTU (or defaultMTU), addressed with inst.Address, configured via
// UAPI with inst.Obfuscation, inst.PrivateKey, opts' AWG 3.0 fields, and one
// UAPI peer per inst.Peers entry. It does not attach a forwarder or start
// relaying traffic -- that's the caller's job (see AttachTCPForwarder /
// AttachUDPHandler) -- which is exactly why a caller that will relay real
// traffic must NOT use this function: see newUnconfiguredDevice's doc
// comment for why, and use newUnconfiguredDevice + Configure instead.
func NewDevice(inst amneziawg.Instance, opts DeviceOptions) (*Device, error) {
	dev, err := newUnconfiguredDevice(inst, opts)
	if err != nil {
		return nil, err
	}
	if err := dev.Configure(inst, opts); err != nil {
		return nil, err
	}
	return dev, nil
}

// newUnconfiguredDevice builds the tun/netstack/device trio but does not
// configure any peers or bring the interface up -- a caller that will relay
// real traffic MUST attach its TCP/UDP handlers (AttachTCPForwarder /
// AttachUDPHandler) against the returned Device.Stack BEFORE calling
// Configure, not after.
//
// This ordering is not a style preference: Configure's IpcSet is what
// starts each configured peer's receive goroutine (amneziawg-go's
// Peer.Start, called from handlePostConfig), and a peer whose handshake
// completes fast enough (e.g. an already-connected client reconnecting
// right as an MTU/address change forces this package's own Manager to
// rebuild the Device) can begin delivering packets into the stack
// immediately -- concurrently with a caller that only calls
// gstack.SetTransportProtocolHandler (AttachTCPForwarder/AttachUDPHandler)
// after Configure returns. A -race CI run caught exactly this as a real
// WARNING: DATA RACE between stack.(*nic).DeliverTransportPacket (the
// peer's receive goroutine, reading the handler table) and
// stack.(*Stack).SetTransportProtocolHandler (the attaching goroutine,
// writing it). See manager.go's ensureLocked rebuild branch for the real
// call order this function exists to support.
func newUnconfiguredDevice(inst amneziawg.Instance, opts DeviceOptions) (*Device, error) {
	addrs, err := hostAddresses(inst.Address)
	if err != nil {
		return nil, fmt.Errorf("amneziawgnet: %w", err)
	}

	mtu := inst.MTU
	if mtu <= 0 {
		mtu = defaultMTU
	}

	tun, gstack, err := createNetTUNWithStack(addrs, mtu)
	if err != nil {
		return nil, fmt.Errorf("amneziawgnet: create netstack: %w", err)
	}

	logger := opts.Logger
	if logger == nil {
		logger = device.NewLogger(device.LogLevelSilent, "")
	}
	dev := device.NewDevice(tun, awgconn.NewDefaultBind(), logger)

	return &Device{Device: dev, Stack: gstack}, nil
}

// Configure applies inst/opts to d via UAPI and brings the interface up.
// Call at most once per Device, and -- for any caller relaying real
// traffic -- only after any AttachTCPForwarder/AttachUDPHandler
// registration against d.Stack (see newUnconfiguredDevice's doc comment
// for why the order matters). Closes d and returns an error if either step
// fails; the caller owns closing anything else it already built against
// d.Stack in that case (e.g. a UDP relay or port-forward set).
func (d *Device) Configure(inst amneziawg.Instance, opts DeviceOptions) error {
	conf, err := buildUAPIConfig(inst, opts)
	if err != nil {
		d.Close()
		return fmt.Errorf("amneziawgnet: %w", err)
	}
	if err := d.IpcSet(conf); err != nil {
		d.Close()
		return fmt.Errorf("amneziawgnet: IpcSet for inbound %d: %w", inst.Id, err)
	}
	if err := d.Up(); err != nil {
		d.Close()
		return fmt.Errorf("amneziawgnet: bring up inbound %d: %w", inst.Id, err)
	}
	return nil
}

// hostAddresses parses each of inst.Address's CIDR strings (e.g.
// "10.8.1.1/24") down to the bare host address the netstack's NIC gets
// configured with -- the interface's own address, not the subnet it routes.
func hostAddresses(addresses []string) ([]netip.Addr, error) {
	out := make([]netip.Addr, 0, len(addresses))
	for _, a := range addresses {
		prefix, err := netip.ParsePrefix(a)
		if err != nil {
			return nil, fmt.Errorf("invalid interface address %q: %w", a, err)
		}
		out = append(out, prefix.Addr())
	}
	return out, nil
}

// buildUAPIConfig renders inst (plus opts' AWG 3.0 fields) as a WireGuard
// UAPI "set" configuration string -- private_key/listen_port/jc.../s1-s4/
// h1-h4/i1 device lines, the AWG 3.0 device lines when opts asks for them,
// then one public_key/preshared_key/allowed_ip block per peer. Field names
// and format match amneziawg-go v3.0.3's device/uapi.go exactly (confirmed
// against its real source during Phase 0 spiking, not just its docs).
func buildUAPIConfig(inst amneziawg.Instance, opts DeviceOptions) (string, error) {
	var b strings.Builder

	privHex, err := wireguard.KeyToHex(inst.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("invalid server private key: %w", err)
	}
	fmt.Fprintf(&b, "private_key=%s\n", privHex)
	fmt.Fprintf(&b, "listen_port=%d\n", inst.ListenPort)
	// replace_peers makes every apply a full resync (matches this package's
	// own Manager.Ensure semantics): peers no longer in inst.Peers are
	// dropped instead of lingering from a previous IpcSet call.
	b.WriteString("replace_peers=true\n")

	o := inst.Obfuscation
	fmt.Fprintf(&b, "jc=%d\njmin=%d\njmax=%d\n", o.Jc, o.Jmin, o.Jmax)
	fmt.Fprintf(&b, "s1=%d\ns2=%d\ns3=%d\ns4=%d\n", o.S1, o.S2, o.S3, o.S4)
	writeHLine(&b, "h1", o.H1)
	writeHLine(&b, "h2", o.H2)
	writeHLine(&b, "h3", o.H3)
	writeHLine(&b, "h4", o.H4)
	if o.I1 != "" {
		fmt.Fprintf(&b, "i1=%s\n", o.I1)
	}

	if opts.HeaderProtectionKey != "" {
		hpHex, err := wireguard.KeyToHex(opts.HeaderProtectionKey)
		if err != nil {
			return "", fmt.Errorf("invalid header protection key: %w", err)
		}
		fmt.Fprintf(&b, "header_protection_key=%s\n", hpHex)
	}
	if opts.ContentPaddingAddition != "" {
		fmt.Fprintf(&b, "content_padding_addition=%s\n", opts.ContentPaddingAddition)
	}
	if opts.RekeyAfterTime != "" {
		fmt.Fprintf(&b, "rekey_after_time=%s\n", opts.RekeyAfterTime)
	}
	if opts.RekeyTimeout != "" {
		fmt.Fprintf(&b, "rekey_timeout=%s\n", opts.RekeyTimeout)
	}
	if opts.RejectAfterTime != "" {
		fmt.Fprintf(&b, "reject_after_time=%s\n", opts.RejectAfterTime)
	}
	if opts.KeepaliveTimeout != "" {
		fmt.Fprintf(&b, "keepalive_timeout=%s\n", opts.KeepaliveTimeout)
	}
	if opts.MaxHandshakeAttempts != "" {
		fmt.Fprintf(&b, "max_handshake_attempts=%s\n", opts.MaxHandshakeAttempts)
	}

	for _, p := range inst.Peers {
		pubHex, err := wireguard.KeyToHex(p.PublicKey)
		if err != nil {
			return "", fmt.Errorf("peer %q: invalid public key: %w", p.Email, err)
		}
		fmt.Fprintf(&b, "public_key=%s\n", pubHex)
		if p.PresharedKey != "" {
			pskHex, err := wireguard.KeyToHex(p.PresharedKey)
			if err != nil {
				return "", fmt.Errorf("peer %q: invalid preshared key: %w", p.Email, err)
			}
			fmt.Fprintf(&b, "preshared_key=%s\n", pskHex)
		}
		for _, allowedIP := range p.AllowedIPs {
			fmt.Fprintf(&b, "allowed_ip=%s\n", allowedIP)
		}
	}

	return b.String(), nil
}

// writeHLine writes an hN UAPI line only when v is set -- an empty H value
// means "let amneziawg-go fall back to its own default," mirroring how
// internal/amneziawg's generateServerConfig treats the same optional field.
func writeHLine(b *strings.Builder, name, v string) {
	if v == "" {
		return
	}
	fmt.Fprintf(b, "%s=%s\n", name, v)
}
