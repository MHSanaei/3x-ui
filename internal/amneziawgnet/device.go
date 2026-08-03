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

// DeviceOptions carries the AmneziaWG 3.0 header-protection fields, kept out
// of amneziawg.Instance/Obfuscation20 deliberately: those are the shared,
// DB-backed types the still-live kernel-module path also reads and writes,
// and 3.0 header protection is a device-wide, strictly opt-in setting (see
// the migration plan's "Reference material" section) that isn't wired into
// that shared schema yet. Zero-value DeviceOptions means classic
// (non-3.0) obfuscation only, matching the kernel-module path's own
// defaults today.
type DeviceOptions struct {
	// HeaderProtectionKey is a base64 32-byte key. Empty disables AWG 3.0
	// header protection entirely. Non-empty requires every one of
	// Obfuscation20.S1-S4 to be >= 12 (amneziawg-go's own HeaderCipherNonceSize
	// requirement) -- IpcSet will reject the config otherwise.
	HeaderProtectionKey string
	// ContentPaddingAddition is a "low-high" range (or a bare integer) per
	// amneziawg-tools' own u16_range_from_string grammar. Empty disables it.
	ContentPaddingAddition string
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

// NewDevice constructs and brings up an embedded AmneziaWG interface for
// inst: a gVisor-backed tun.Device sized to inst.MTU (or defaultMTU),
// addressed with inst.Address, configured via UAPI with inst.Obfuscation,
// inst.PrivateKey, opts' AWG 3.0 fields, and one UAPI peer per inst.Peers
// entry. It does not attach a forwarder or start relaying traffic --
// that's the caller's job (see AttachTCPForwarder / AttachUDPHandler),
// keeping this constructor usable both for a real relay and for a plain
// mechanical test.
func NewDevice(inst amneziawg.Instance, opts DeviceOptions) (*Device, error) {
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

	conf, err := buildUAPIConfig(inst, opts)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("amneziawgnet: %w", err)
	}
	if err := dev.IpcSet(conf); err != nil {
		dev.Close()
		return nil, fmt.Errorf("amneziawgnet: IpcSet for inbound %d: %w", inst.Id, err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("amneziawgnet: bring up inbound %d: %w", inst.Id, err)
	}

	return &Device{Device: dev, Stack: gstack}, nil
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
