package amneziawgnet

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func TestEchoPolicyPreservesLocalTunnelHealth(t *testing.T) {
	for _, dst := range []string{"192.0.2.1", "192.0.2.9"} {
		t.Run(dst, func(t *testing.T) {
			tun, _, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.9")}, 1420)
			if err != nil {
				t.Fatal(err)
			}
			defer tun.Close()
			stackTun := tun.(*stackTun)
			calls := 0
			stackTun.echoHandler = func([]byte) { calls++ }
			packet := echoTestPacket("192.0.2.2", dst, 123, 17, []byte("tunnel health"))
			if _, err := tun.Write([][]byte{packet}, 0); err != nil {
				t.Fatal(err)
			}
			select {
			case view := <-stackTun.incomingPacket:
				reply := bytes.Clone(view.AsSlice())
				view.Release()
				// gVisor owns IPv4 id/flags; addresses and the ICMP reply must survive exactly.
				if len(reply) != len(packet) || !bytes.Equal(reply[12:16], packet[16:20]) || !bytes.Equal(reply[16:20], packet[12:16]) || reply[20] != 0 || !bytes.Equal(reply[24:], packet[24:]) || echoChecksum(reply[:20]) != 0 || echoChecksum(reply[20:]) != 0 {
					t.Fatalf("invalid local reply: %x", reply)
				}
			case <-time.After(time.Second):
				t.Fatal("disabled forwarding broke the local tunnel-health reply")
			}
			if calls != 0 {
				t.Fatal("local health used the forwarding policy or host socket")
			}
		})
	}
}

func TestEchoInterceptionPreservesNonEchoICMP(t *testing.T) {
	for _, dst := range []string{"192.0.2.1", "198.51.100.7"} {
		t.Run(dst, func(t *testing.T) {
			tun, stk, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("192.0.2.1")}, 1420)
			if err != nil {
				t.Fatal(err)
			}
			defer tun.Close()
			stk.SetPromiscuousMode(1, true)
			stackTun := tun.(*stackTun)
			calls := 0
			stackTun.echoHandler = func([]byte) { calls++ }
			inner := echoTestPacket("192.0.2.2", "198.51.100.8", 1, 2, nil)
			packet := echoTestPacket("192.0.2.2", dst, 0, 0, inner)
			packet[20], packet[21], packet[22], packet[23] = 3, 3, 0, 0
			binary.BigEndian.PutUint16(packet[22:24], echoChecksum(packet[20:]))
			if _, err := tun.Write([][]byte{packet}, 0); err != nil {
				t.Fatal(err)
			}
			if got := stk.Stats().ICMP.V4.PacketsReceived.DstUnreachable.Value(); got != 1 {
				t.Fatalf("stack received %d ICMP errors, want 1", got)
			}
			if calls != 0 {
				t.Fatal("ICMP error used the echo relay")
			}
		})
	}
}

func TestEchoInterceptionNeverFabricatesForeignReply(t *testing.T) {
	tun, stk, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("192.0.2.1")}, 1420)
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()
	stk.SetPromiscuousMode(1, true)
	stackTun := tun.(*stackTun)
	calls := 0
	stackTun.echoHandler = func([]byte) { calls++ }
	// A same-subnet destination is still foreign unless it is a configured interface address.
	for _, dst := range []string{"192.0.2.99", "198.51.100.7"} {
		packet := echoTestPacket("192.0.2.2", dst, 123, 17, []byte("unreachable"))
		if _, err := tun.Write([][]byte{packet}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 || stk.Stats().ICMP.V4.PacketsReceived.EchoRequest.Value() != 0 || len(stackTun.incomingPacket) != 0 {
		t.Fatal("foreign echo escaped the forwarding policy")
	}
}

func TestEchoFragmentsCannotBypassRemotePolicy(t *testing.T) {
	tun, stk, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("192.0.2.1")}, 1420)
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()
	stk.SetPromiscuousMode(1, true)
	stackTun := tun.(*stackTun)
	relay, socket, replies := testEchoRelay(t, func(int, amneziawg.Peer, netip.Addr, netip.Addr, int64, int64, string) (string, error) {
		t.Fatal("fragment entered remote authorization")
		return "", nil
	})
	stackTun.echoHandler = relay.process
	packet := echoTestPacket("192.0.2.2", "198.51.100.7", 123, 17, bytes.Repeat([]byte{0}, 32))
	for _, fragment := range []struct {
		start, end int
		flags      uint16
	}{{0, 16, 0x2000}, {16, len(packet) - 20, 2}} {
		p := append(bytes.Clone(packet[:20]), packet[20+fragment.start:20+fragment.end]...)
		binary.BigEndian.PutUint16(p[2:4], uint16(len(p)))
		binary.BigEndian.PutUint16(p[6:8], fragment.flags)
		p[10], p[11] = 0, 0
		binary.BigEndian.PutUint16(p[10:12], echoChecksum(p[:20]))
		if _, err := tun.Write([][]byte{p}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if stk.Stats().ICMP.V4.PacketsReceived.EchoRequest.Value() != 0 || len(stackTun.incomingPacket) != 0 || len(socket.writes) != 0 || len(replies) != 0 {
		t.Fatal("fragmented Echo bypassed the remote policy")
	}
}
