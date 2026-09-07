package amneziawgnet

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// BenchmarkStackTunWrite measures the upload path's per-packet cost: one
// decrypted packet handed from amneziawg-go into the gVisor stack.
func BenchmarkStackTunWrite(b *testing.B) {
	tun := &stackTun{ep: channel.New(tunQueueDepth, 1420, ""), mtu: 1420}
	defer tun.ep.Close()

	packet := make([]byte, 1400)
	packet[0] = 0x45
	bufs := [][]byte{packet}

	b.SetBytes(int64(len(packet)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := tun.Write(bufs, 0); err != nil {
			b.Fatalf("Write: %v", err)
		}
	}
}

// BenchmarkStackTunRead measures the download path's per-packet cost: one
// packet drained out of the stack for amneziawg-go to encrypt.
func BenchmarkStackTunRead(b *testing.B) {
	tun := &stackTun{incomingPacket: make(chan *buffer.View, tunQueueDepth)}
	packet := make([]byte, 1400)
	buf := [][]byte{make([]byte, 2048)}
	sizes := make([]int, 1)

	b.SetBytes(int64(len(packet)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		tun.incomingPacket <- buffer.NewViewWithData(packet)
		if _, err := tun.Read(buf, sizes, 0); err != nil {
			b.Fatalf("Read: %v", err)
		}
	}
}

// BenchmarkUDPDatagramDelivery measures one datagram travelling the whole
// inbound path: stack injection, routing, and the UDP transport handler.
func BenchmarkUDPDatagramDelivery(b *testing.B) {
	tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.78.0.1")}, 1420)
	if err != nil {
		b.Fatalf("createNetTUNWithStack: %v", err)
	}
	defer tun.Close()

	src := netip.MustParseAddrPort("10.78.0.2:40000")
	dst := netip.MustParseAddrPort("10.78.9.9:5353")
	payload := make([]byte, 1024)
	AttachUDPHandler(gstack, func(netip.AddrPort, netip.AddrPort, []byte) {})

	bufs := [][]byte{udpDatagram(src, dst, payload)}
	st := tun.(*stackTun)

	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := st.Write(bufs, 0); err != nil {
			b.Fatalf("Write: %v", err)
		}
	}
}

// Two destinations the benchmark forwarder tells apart: one drains what the
// client sends, the other streams at the client. Neither is routed anywhere.
const (
	benchDiscardPort = 9001
	benchSourcePort  = 9002
)

// benchTunnel is a live AmneziaWG pair -- this package's server Device and a
// stock amneziawg-go client -- talking real encrypted UDP over loopback.
type benchTunnel struct {
	clientNet *netstack.Net
	closeFn   func()
}

// newBenchTunnel brings up both devices and blocks until the handshake has
// actually completed, so no setup cost lands inside the measured loop.
func newBenchTunnel(b *testing.B, listenPort int, serverAddr, clientAddr string) *benchTunnel {
	b.Helper()
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		b.Fatalf("server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		b.Fatalf("client keypair: %v", err)
	}

	inst := amneziawg.Instance{
		Id:            90,
		InterfaceName: "awgbench",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{serverAddr + "/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{{
			Email:      "bench@example.com",
			PublicKey:  clientPub,
			AllowedIPs: []string{clientAddr + "/32"},
		}},
	}

	dev, err := newUnconfiguredDevice(inst, DeviceOptions{})
	if err != nil {
		b.Fatalf("newUnconfiguredDevice: %v", err)
	}
	AttachTCPForwarder(dev.Stack, func(conn *gonet.TCPConn, dest netip.AddrPort) {
		defer conn.Close()
		switch dest.Port() {
		case benchDiscardPort:
			_, _ = io.Copy(io.Discard, conn)
		case benchSourcePort:
			chunk := make([]byte, 64<<10)
			for {
				if _, err := conn.Write(chunk); err != nil {
					return
				}
			}
		}
	})
	if err := dev.Configure(inst, DeviceOptions{}); err != nil {
		b.Fatalf("Configure: %v", err)
	}

	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr(clientAddr)},
		[]netip.Addr{netip.MustParseAddr("1.1.1.1")}, 1420)
	if err != nil {
		b.Fatalf("client CreateNetTUN: %v", err)
	}
	clientDev := device.NewDevice(clientTun, awgconn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))

	clientPrivHex, err := wireguard.KeyToHex(clientPriv)
	if err != nil {
		b.Fatalf("client key to hex: %v", err)
	}
	serverPubHex, err := wireguard.KeyToHex(serverPub)
	if err != nil {
		b.Fatalf("server key to hex: %v", err)
	}
	conf := fmt.Sprintf(
		"private_key=%s\njc=4\njmin=40\njmax=70\ns1=20\ns2=30\ns3=20\ns4=20\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n",
		clientPrivHex, serverPubHex, listenPort)
	if err := clientDev.IpcSet(conf); err != nil {
		b.Fatalf("client IpcSet: %v", err)
	}
	if err := clientDev.Up(); err != nil {
		b.Fatalf("client Up: %v", err)
	}

	t := &benchTunnel{clientNet: clientNet, closeFn: func() {
		clientDev.Close()
		dev.Close()
	}}
	// Prove the handshake really completed before anything is timed.
	probe := t.dial(b, netip.MustParseAddrPort(fmt.Sprintf("%s:%d", serverAddr, benchDiscardPort)))
	probe.Close()
	return t
}

// dial opens one tunnelled connection, retrying while the handshake settles.
func (t *benchTunnel) dial(b *testing.B, dest netip.AddrPort) *gonet.TCPConn {
	b.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		conn, err := t.clientNet.DialContextTCPAddrPort(ctx, dest)
		cancel()
		if err == nil {
			return conn
		}
		if time.Now().After(deadline) {
			b.Fatalf("dial %v through tunnel: %v", dest, err)
		}
	}
}

// BenchmarkTunnelThroughput is the end-to-end number: real bytes through a
// real handshaked AmneziaWG tunnel, in both directions.
func BenchmarkTunnelThroughput(b *testing.B) {
	const chunkSize = 64 << 10
	const serverAddr = "10.203.0.1"
	tun := newBenchTunnel(b, 58714, serverAddr, "10.203.0.2")
	defer tun.closeFn()

	b.Run("upload", func(b *testing.B) {
		conn := tun.dial(b, netip.MustParseAddrPort(fmt.Sprintf("%s:%d", serverAddr, benchDiscardPort)))
		defer conn.Close()
		chunk := make([]byte, chunkSize)
		b.SetBytes(chunkSize)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := conn.Write(chunk); err != nil {
				b.Fatalf("upload write: %v", err)
			}
		}
	})

	b.Run("download", func(b *testing.B) {
		conn := tun.dial(b, netip.MustParseAddrPort(fmt.Sprintf("%s:%d", serverAddr, benchSourcePort)))
		defer conn.Close()
		chunk := make([]byte, chunkSize)
		b.SetBytes(chunkSize)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := io.ReadFull(conn, chunk); err != nil {
				b.Fatalf("download read: %v", err)
			}
		}
	})
}
