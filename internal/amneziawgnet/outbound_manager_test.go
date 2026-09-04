package amneziawgnet

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// egressPortBound reports whether 127.0.0.1:<EgressBasePort> accepts TCP.
func egressPortBound(t *testing.T) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", itoa(int(EgressBasePort))), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// newTestOutboundDesired builds one runnable outbound desired state.
func newTestOutboundDesired(t *testing.T, tag string) OutboundDesired {
	t.Helper()
	priv, _, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return OutboundDesired{
		Instance: amneziawg.OutboundInstance{
			Tag:        tag,
			Address:    []string{"10.204.0.1/24"},
			MTU:        1420,
			PrivateKey: priv,
			ListenPort: 0,
		},
	}
}

// TestOutboundManagerReconcileEmptyDesiredClosesEgress verifies that an empty
// desired set tears down interfaces and releases 127.0.0.1:64900.
func TestOutboundManagerReconcileEmptyDesiredClosesEgress(t *testing.T) {
	m := &OutboundManager{iface: map[string]*managedOutbound{}}
	defer m.Reconcile(nil)

	// Other tests in this package may leave the process-wide egress
	// singleton bound; converge to a known-free state before pinning.
	GetEgressServer().Close()
	if egressPortBound(t) {
		t.Fatal("egress port still bound after Close; Close() failed to release it")
	}

	// Non-empty: listener must come up.
	d := newTestOutboundDesired(t, "t1")
	m.Reconcile([]OutboundDesired{d})
	if !egressPortBound(t) {
		t.Fatal("egress port not bound after Reconcile with a desired outbound")
	}

	// Empty: listener must be released so other listeners can take the port.
	m.Reconcile(nil)
	if egressPortBound(t) {
		t.Fatal("egress port still bound after Reconcile(nil)")
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", itoa(int(EgressBasePort))))
	if err != nil {
		t.Fatalf("egress port must be free after Reconcile(nil): %v", err)
	}
	ln.Close()

	// Back to non-empty and empty again: Close/Listen must be repeatable.
	m.Reconcile([]OutboundDesired{d})
	if !egressPortBound(t) {
		t.Fatal("egress port not re-bound after a second non-empty Reconcile")
	}
	m.Reconcile(nil)
	if egressPortBound(t) {
		t.Fatal("egress port still bound after a second Reconcile(nil)")
	}
}

// TestEgressServerCloseDuringConcurrentAccepts ensures Close during
// concurrent accepts shuts down cleanly without hanging wg.Wait().
func TestEgressServerCloseDuringConcurrentAccepts(t *testing.T) {
	srv := GetEgressServer()
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	var clientWg sync.WaitGroup
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", itoa(int(EgressBasePort))), 50*time.Millisecond)
				if err == nil {
					clientWg.Add(1)
					go func(conn net.Conn) {
						defer clientWg.Done()
						time.Sleep(20 * time.Millisecond)
						conn.Close()
					}(c)
				}
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	time.Sleep(20 * time.Millisecond)
	closeChan := make(chan struct{})
	go func() {
		srv.Close()
		close(closeChan)
	}()

	select {
	case <-closeChan:
	case <-time.After(3 * time.Second):
		t.Fatal("srv.Close() hung waiting for connection handlers to exit")
	}
	close(stop)
	<-done
	clientWg.Wait()
}
