package amneziawgnet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"golang.org/x/net/icmp"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func TestEchoEncryptedAWGToRealICMP(t *testing.T) {
	target := os.Getenv("AWG_ECHO_E2E_TARGET")
	if target == "" {
		t.Skip("set AWG_ECHO_E2E_TARGET to an isolated namespace address")
	}
	dst := netip.MustParseAddr(target)
	observer, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close()
	seen := make(chan []byte, 256)
	go func() {
		buf := make([]byte, 65536)
		for {
			n, addr, e := observer.ReadFrom(buf)
			if e != nil {
				return
			}
			ip, ok := addr.(*net.IPAddr)
			if ok && ip.IP.String() == target && n >= 8 && buf[0] == 8 {
				select {
				case seen <- bytes.Clone(buf[:n]):
				default:
				}
			}
		}
	}()
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	var enabled atomic.Bool
	enabled.Store(true)
	var up, down atomic.Int64
	gate := func(_ int, p amneziawg.Peer, src, target netip.Addr, u, d int64, version string) (string, error) {
		if !enabled.Load() || p.PublicKey != clientPub || src.String() != "192.0.2.2" || (target != dst && target.String() != "198.18.0.2") || (version != "" && version != "synthetic-v1") {
			return "", errors.New("denied")
		}
		up.Add(u)
		down.Add(d)
		return "synthetic-v1", nil
	}
	port := freePort(t)
	inst := amneziawg.Instance{Id: 987, ListenPort: port, PrivateKey: serverPriv, PublicKey: serverPub, Address: []string{"192.0.2.1/24"}, MTU: 1420, Peers: []amneziawg.Peer{{Email: "synthetic-e2e", PublicKey: clientPub, AllowedIPs: []string{"192.0.2.2/32"}}}}
	manager := &Manager{ifaces: map[int]*managed{}}
	if err := manager.Ensure(Desired{Instance: inst, EchoGate: gate}); err != nil {
		t.Fatal(err)
	}
	defer manager.StopAll()
	tun, clientNet, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr("192.0.2.2")}, nil, 1420)
	if err != nil {
		t.Fatal(err)
	}
	client := device.NewDevice(tun, newListenBind("127.0.0.1"), device.NewLogger(device.LogLevelSilent, ""))
	defer client.Close()
	privHex, err := wireguard.KeyToHex(clientPriv)
	if err != nil {
		t.Fatal(err)
	}
	pubHex, err := wireguard.KeyToHex(serverPub)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.IpcSet(fmt.Sprintf("private_key=%s\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n", privHex, pubHex, port)); err != nil {
		t.Fatal(err)
	}
	if err := client.Up(); err != nil {
		t.Fatal(err)
	}
	conn, err := clientNet.DialPingAddr(netip.MustParseAddr("192.0.2.2"), dst)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	payload := []byte("real echo through encrypted AWG")
	msg := make([]byte, 8+len(payload))
	msg[0] = 8
	binary.BigEndian.PutUint16(msg[4:6], 1234)
	binary.BigEndian.PutUint16(msg[6:8], 55)
	copy(msg[8:], payload)
	binary.BigEndian.PutUint16(msg[2:4], echoChecksum(msg))
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(msg); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 2048)
	n, err := conn.Read(reply)
	if err != nil {
		t.Fatal(err)
	}
	// gVisor's ping socket owns the wire identifier; payload and sequence must still survive exactly.
	if n != len(msg) || reply[0] != 0 || binary.BigEndian.Uint16(reply[6:8]) != 55 || !bytes.Equal(reply[8:n], payload) {
		t.Fatalf("bad echo reply: %x", reply[:n])
	}
	select {
	case wire := <-seen:
		if !bytes.Equal(wire[8:8+len(payload)], payload) || len(wire) != len(msg)+16 {
			t.Fatal("target did not receive original payload plus nonce")
		}
	case <-time.After(time.Second):
		t.Fatal("no real request at target")
	}
	want := int64(len(msg) + 20)
	if up.Load() != want || down.Load() != want {
		t.Fatalf("inner-IP traffic %d/%d want %d/%d", up.Load(), down.Load(), want, want)
	}
	enabled.Store(false)
	if err := conn.SetDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(msg); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Read(reply); err == nil {
		t.Fatal("disabled policy fabricated a reply")
	}
	select {
	case <-seen:
		t.Fatal("disabled policy sent host ICMP")
	default:
	}
	if up.Load() != want || down.Load() != want {
		t.Fatal("disabled probe was charged")
	}
	// Even the server's own netstack address must time out when ping is denied.
	own, err := clientNet.DialPingAddr(netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1"))
	if err != nil {
		t.Fatal(err)
	}
	defer own.Close()
	_ = own.SetDeadline(time.Now().Add(200 * time.Millisecond))
	_, _ = own.Write(msg)
	if _, err := own.Read(reply); err == nil {
		t.Fatal("gVisor local Echo bypassed the gate")
	}
	enabled.Store(true)
	missing, err := clientNet.DialPingAddr(netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("198.18.0.2"))
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Close()
	_ = missing.SetDeadline(time.Now().Add(200 * time.Millisecond))
	_, _ = missing.Write(msg)
	if _, err := missing.Read(reply); err == nil {
		t.Fatal("unreachable destination reported up")
	}
	state, err := client.IpcGet()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(state, "last_handshake_time_sec=") {
		t.Fatal("no encrypted handshake")
	}
}
