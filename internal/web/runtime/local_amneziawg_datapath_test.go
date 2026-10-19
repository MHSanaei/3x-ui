package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// datapathInbound builds a minimal AmneziaWG inbound, kernel mode on or off.
func datapathInbound(t *testing.T, id, port int, kernelMode bool) *model.Inbound {
	t.Helper()
	srvPriv, srvPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	_, peerPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate peer keypair: %v", err)
	}
	settings := fmt.Sprintf(
		`{"server":{"privateKey":%q,"publicKey":%q,"subnetIp":"10.209.0.0","subnetCidr":24,"mtu":1420,`+
			`"jc":4,"jmin":40,"jmax":70,"s1":20,"s2":30,"s3":20,"s4":20,"kernelMode":%t},`+
			`"clients":[{"email":"peer@example.test","enable":true,"publicKey":%q,"allowedIPs":["10.209.0.2/32"]}]}`,
		srvPriv, srvPub, kernelMode, peerPub)
	return &model.Inbound{
		Id:       id,
		Enable:   true,
		Protocol: model.AmneziaWG,
		Port:     port,
		Tag:      fmt.Sprintf("inbound-%d", id),
		Settings: settings,
	}
}

func newTestLocal() *Local {
	return NewLocal(LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}})
}

// Both datapaths bind the inbound's own UDP port, so an embedded device raised
// here collides with the kernel interface the reconcile tick brings up.
func TestAddInboundKeepsKernelModeOffTheEmbeddedDatapath(t *testing.T) {
	ib := datapathInbound(t, 4181, 58731, true)
	t.Cleanup(func() { amneziawgnet.GetManager().Remove(ib.Id) })

	// The kernel side wants awg-quick and root, so it is expected to fail on a
	// dev box; what matters is that amneziawgnet was never asked to take over.
	_ = newTestLocal().AddInbound(context.Background(), ib)

	if _, _, ok := amneziawgnet.GetManager().Lookup(ib.Id); ok {
		t.Fatal("a kernel-mode inbound created an embedded device")
	}
}

// The other half: without the toggle the dispatch must still reach
// amneziawgnet, or it would be a blanket opt-out rather than a routing choice.
func TestAddInboundStillUsesTheEmbeddedDatapath(t *testing.T) {
	ib := datapathInbound(t, 4182, 58732, false)
	t.Cleanup(func() { amneziawgnet.GetManager().Remove(ib.Id) })

	if err := newTestLocal().AddInbound(context.Background(), ib); err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	if _, _, ok := amneziawgnet.GetManager().Lookup(ib.Id); !ok {
		t.Fatal("an embedded inbound did not create a device")
	}
}
