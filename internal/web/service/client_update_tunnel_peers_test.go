package service

import (
	"encoding/base64"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func wgTestKeypair(t *testing.T, seed byte) (priv, pub string) {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = seed
	}
	priv = base64.StdEncoding.EncodeToString(raw)
	pub, err := wgutil.PublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatalf("derive public key: %v", err)
	}
	return priv, pub
}

func inboundPeer(t *testing.T, inboundSvc *InboundService, ibId int, email string) model.Client {
	t.Helper()
	ib, err := inboundSvc.GetInbound(ibId)
	if err != nil {
		t.Fatalf("GetInbound %d: %v", ibId, err)
	}
	clients, err := inboundSvc.GetClients(ib)
	if err != nil {
		t.Fatalf("GetClients %d: %v", ibId, err)
	}
	for i := range clients {
		if clients[i].Email == email {
			return clients[i]
		}
	}
	t.Fatalf("email %q not found on inbound %d", email, ibId)
	return model.Client{}
}

// A client on several WireGuard inbounds is several independent peers, each
// with its own keypair and tunnel address. The edit form can only carry one
// field set, so a save that broadcasts it leaves every peer but one with keys
// and an address belonging to a different node, breaking those tunnels.
func TestUpdateDoesNotBroadcastPeerCredentialsAcrossTunnelInbounds(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	const email = "multi@wg"
	privA, pubA := wgTestKeypair(t, 0x11)
	privB, pubB := wgTestKeypair(t, 0x22)

	peerA := model.Client{
		Email: email, SubID: "sub-multi", Enable: true,
		PrivateKey: privA, PublicKey: pubA, AllowedIPs: []string{"10.10.151.5/32"},
	}
	peerB := model.Client{
		Email: email, SubID: "sub-multi", Enable: true,
		PrivateKey: privB, PublicKey: pubB, AllowedIPs: []string{"10.10.152.5/32"},
	}

	ibA := mkInbound(t, 51821, model.WireGuard, clientsSettings(t, []model.Client{peerA}))
	if err := svc.SyncInbound(nil, ibA.Id, []model.Client{peerA}); err != nil {
		t.Fatalf("seed inbound A linkage: %v", err)
	}
	ibB := mkInbound(t, 51822, model.WireGuard, clientsSettings(t, []model.Client{peerB}))
	if err := svc.SyncInbound(nil, ibB.Id, []model.Client{peerB}); err != nil {
		t.Fatalf("seed inbound B linkage: %v", err)
	}
	recId := lookupClientRecord(t, email).Id

	// What the client edit form sends: inbound A's peer fields, once, for
	// a save that only meant to change an unrelated field.
	updated := model.Client{
		Email: email, Enable: true, Comment: "renamed",
		PrivateKey: privA, PublicKey: pubA, AllowedIPs: []string{"10.10.151.5/32"},
	}
	if _, err := svc.Update(inboundSvc, recId, updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	gotB := inboundPeer(t, inboundSvc, ibB.Id, email)
	if gotB.PrivateKey != privB || gotB.PublicKey != pubB {
		t.Fatalf("inbound B peer keys were overwritten with inbound A's: private=%q public=%q", gotB.PrivateKey, gotB.PublicKey)
	}
	if len(gotB.AllowedIPs) != 1 || gotB.AllowedIPs[0] != "10.10.152.5/32" {
		t.Fatalf("inbound B AllowedIPs = %v, want unchanged [10.10.152.5/32]", gotB.AllowedIPs)
	}

	gotA := inboundPeer(t, inboundSvc, ibA.Id, email)
	if gotA.PrivateKey != privA || len(gotA.AllowedIPs) != 1 || gotA.AllowedIPs[0] != "10.10.151.5/32" {
		t.Fatalf("inbound A peer must keep its own values, got %+v", gotA)
	}
}
