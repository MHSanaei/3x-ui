package amneziawgnet

import (
	"bufio"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// ClientDiagnostic is one configured peer's live state, cross-referenced
// from the running Device's own UAPI dump against the peer list the caller
// supplies. A peer that has never handshaked still appears here (with a
// zero LastHandshake) rather than being silently absent, so an admin can
// tell "misconfigured client" apart from "client just hasn't connected".
type ClientDiagnostic struct {
	Email         string
	LastHandshake time.Time
	RxBytes       uint64
	TxBytes       uint64
	Endpoint      string
	// AllowedIPs is comma-joined, from the running Device's own UAPI dump
	// when it has ever handshaked (so a re-IP is reflected immediately);
	// falls back to the peer's configured AllowedIPs otherwise.
	AllowedIPs string
}

// Connected reports whether this client has ever completed a handshake.
func (c ClientDiagnostic) Connected() bool {
	return !c.LastHandshake.IsZero()
}

// Diagnostics is a read-only snapshot of one embedded AmneziaWG inbound's
// live state. Gathering it can never itself change anything -- it only
// reads the already-running Device's own UAPI dump, never writes to it.
type Diagnostics struct {
	Running    bool
	ListenPort int
	Clients    []ClientDiagnostic
}

// Diagnose builds a live snapshot for inbound id, cross-referenced against
// peers (the inbound's currently configured client list -- the caller
// supplies this since amneziawgnet has no DB access of its own). Running
// stays false (with an empty Clients list) when there's no managed
// instance for this id right now -- disabled, not yet reconciled, or never
// started -- which is itself useful information for an admin, not an error.
func Diagnose(id int, peers []amneziawg.Peer) Diagnostics {
	dev, _, ok := GetManager().Lookup(id)
	if !ok {
		return Diagnostics{}
	}
	return diagnoseDevice(dev, peers)
}

func diagnoseDevice(dev *Device, peers []amneziawg.Peer) Diagnostics {
	diag := Diagnostics{Running: true}

	dump, err := dev.IpcGet()
	if err != nil {
		return diag
	}
	listenPort, states := parseUAPIDump(dump)
	diag.ListenPort = listenPort

	diag.Clients = make([]ClientDiagnostic, 0, len(peers))
	for _, p := range peers {
		hexKey, err := wireguard.KeyToHex(p.PublicKey)
		if err != nil {
			continue
		}
		st := states[hexKey]
		cd := ClientDiagnostic{Email: p.Email, RxBytes: st.rxBytes, TxBytes: st.txBytes, Endpoint: st.endpoint}
		if st.lastHandshakeSec > 0 {
			cd.LastHandshake = time.Unix(st.lastHandshakeSec, 0)
		}
		if len(st.allowedIPs) > 0 {
			cd.AllowedIPs = strings.Join(st.allowedIPs, ", ")
		} else {
			cd.AllowedIPs = strings.Join(p.AllowedIPs, ", ")
		}
		diag.Clients = append(diag.Clients, cd)
	}
	return diag
}

type peerUAPIState struct {
	lastHandshakeSec int64
	rxBytes, txBytes uint64
	endpoint         string
	allowedIPs       []string
}

// parseUAPIDump reads a Device.IpcGet() text dump: device-level keys first
// (only listen_port matters here), then one block per peer starting with
// its own public_key= line. Keyed by lowercase-hex public key, matching
// the hex form IpcGet itself emits (see wireguard.KeyToHex for the same
// base64-to-hex conversion applied to our own stored keys before lookup).
func parseUAPIDump(dump string) (listenPort int, states map[string]peerUAPIState) {
	states = make(map[string]peerUAPIState)
	var current string

	scanner := bufio.NewScanner(strings.NewReader(dump))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		switch key {
		case "listen_port":
			listenPort, _ = strconv.Atoi(value)
		case "public_key":
			current = value
			states[current] = peerUAPIState{}
		case "last_handshake_time_sec":
			st := states[current]
			st.lastHandshakeSec, _ = strconv.ParseInt(value, 10, 64)
			states[current] = st
		case "rx_bytes":
			st := states[current]
			st.rxBytes, _ = strconv.ParseUint(value, 10, 64)
			states[current] = st
		case "tx_bytes":
			st := states[current]
			st.txBytes, _ = strconv.ParseUint(value, 10, 64)
			states[current] = st
		case "endpoint":
			st := states[current]
			st.endpoint = value
			states[current] = st
		case "allowed_ip":
			st := states[current]
			st.allowedIPs = append(st.allowedIPs, value)
			states[current] = st
		}
	}
	return listenPort, states
}
