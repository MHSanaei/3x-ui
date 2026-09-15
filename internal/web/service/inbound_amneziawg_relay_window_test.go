package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// awgRelayWindowSettings builds an AmneziaWG settings blob AddInbound accepts:
// real X25519 keys, one enabled peer, and an email unique to tag.
func awgRelayWindowSettings(t *testing.T, tag string) string {
	t.Helper()
	_, clientPub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}
	return `{"server":{"privateKey":"` + awgTestPrivateKey + `","publicKey":"` + awgTestPublicKey +
		`","subnetIp":"10.8.1.0","subnetCidr":24},"clients":[{"email":"` + tag + `@relay-window","enable":true,"publicKey":"` +
		clientPub + `","allowedIPs":["10.8.1.2/32"]}]}`
}

// pushInboundIDSequence makes the next inbounds insert land on nextID, standing
// in for a long-lived database whose AUTOINCREMENT counter has climbed there.
func pushInboundIDSequence(t *testing.T, nextID int) {
	t.Helper()
	// The counter is a sqlite_sequence row, so this has no PostgreSQL equivalent.
	if database.IsPostgres() {
		t.Skip("the inbounds AUTOINCREMENT counter is a SQLite row")
	}
	res := database.GetDB().Exec("UPDATE sqlite_sequence SET seq = ? WHERE name = ?", nextID-1, "inbounds")
	if res.Error != nil {
		t.Fatalf("push the inbounds sequence to %d: %v", nextID, res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("inbounds has no AUTOINCREMENT counter row to push (%d rows updated)", res.RowsAffected)
	}
}

func addAmneziaWGInbound(t *testing.T, tag string, port int, enable bool) *model.Inbound {
	t.Helper()
	created, _, err := (&InboundService{}).AddInbound(&model.Inbound{
		Tag:      tag,
		Enable:   enable,
		Listen:   "0.0.0.0",
		Port:     port,
		Protocol: model.AmneziaWG,
		Settings: awgRelayWindowSettings(t, tag),
	})
	if err != nil {
		t.Fatalf("AddInbound(%s): %v", tag, err)
	}
	return created
}

// An id past the slot count used to be refused outright, which capped a
// database at 435 AmneziaWG inbounds for its entire life (#6537).
func TestAddInbound_AmneziawgPastTheRelayPortWindowStillCreates(t *testing.T) {
	setupConflictDB(t)

	// Self-check: the low-id path must work, or the assertion below could pass
	// because the fixture never created an AmneziaWG inbound at all.
	addAmneziaWGInbound(t, "awg-low-id", 51820, true)

	pushInboundIDSequence(t, 70001)
	created := addAmneziaWGInbound(t, "awg-past-window", 51821, true)
	if created.Id < 436 {
		t.Fatalf("fixture: inbound id %d is still inside the old window", created.Id)
	}
	if port := amneziawgnet.SOCKSPortForInbound(created.Id); port < amneziawgnet.SOCKSBasePort+1 || port > 65535 {
		t.Fatalf("inbound %d derived relay port %d, outside %d..65535", created.Id, port, amneziawgnet.SOCKSBasePort+1)
	}
}

// Wrapping ids makes the id -> relay-port map non-injective, so a create can
// land on a port an existing inbound's relay already owns.
func TestAddInbound_AmneziawgRefusesAClaimedRelayPort(t *testing.T) {
	for _, blockerEnabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("blocker enabled=%t", blockerEnabled), func(t *testing.T) {
			setupConflictDB(t)
			blocker := addAmneziaWGInbound(t, "awg-blocker", 51820, blockerEnabled)

			// One slot-window further on is the id that derives the blocker's port.
			collidingID := blocker.Id + 435
			pushInboundIDSequence(t, collidingID)

			_, _, err := (&InboundService{}).AddInbound(&model.Inbound{
				Tag:      "awg-collides",
				Enable:   true,
				Listen:   "0.0.0.0",
				Port:     51821,
				Protocol: model.AmneziaWG,
				Settings: awgRelayWindowSettings(t, "awg-collides"),
			})
			if err == nil {
				t.Fatalf("inbound %d derives relay port %d, already owned by %q; the create must be refused",
					collidingID, amneziawgnet.SOCKSPortForInbound(blocker.Id), blocker.Tag)
			}
			if !strings.Contains(err.Error(), blocker.Tag) {
				t.Fatalf("the conflict must name the inbound owning the port, got %v", err)
			}
			// The blocker's own port is its WireGuard one, so without this the
			// message reads as if that inbound listened on an unrelated port.
			if !strings.Contains(err.Error(), "relay port") {
				t.Fatalf("the refusal must say the port is an automatic relay one, got %v", err)
			}
		})
	}
}

// Wrapping makes id -> relay port non-injective, so an edit landing on a slot a
// local inbound already owns has to be refused: the create guard never sees it.
func TestCheckPortConflict_LocalAmneziawgRelayCollisionBlocksTheEdit(t *testing.T) {
	setupConflictDB(t)
	blocker := addAmneziaWGInbound(t, "awg-blocker", 51820, true)

	local := &model.Inbound{
		Tag:      "awg-edited",
		Enable:   true,
		Listen:   "0.0.0.0",
		Port:     51821,
		Protocol: model.AmneziaWG,
		Settings: awgRelayWindowSettings(t, "awg-edited"),
	}
	collidingID := blocker.Id + 435

	got, err := (&InboundService{}).checkPortConflict(local, collidingID)
	if err != nil {
		t.Fatalf("checkPortConflict: %v", err)
	}
	if got == nil {
		t.Fatalf("id %d derives relay port %d, already owned by %q; the save must be refused",
			collidingID, amneziawgnet.SOCKSPortForInbound(blocker.Id), blocker.Tag)
	}
	if !strings.Contains(got.String(), blocker.Tag) {
		t.Fatalf("the conflict must name the inbound owning the port, got %q", got.String())
	}
}

// A disabled row still owns the relay slot its id derives: SetInboundEnable
// flips the column with no port check, so enabling it later would break Xray.
func TestCheckPortConflict_DisabledAmneziawgStillOwnsItsRelaySlot(t *testing.T) {
	setupConflictDB(t)
	owner := addAmneziaWGInbound(t, "awg-disabled", 51820, false)
	relayPort := amneziawgnet.SOCKSPortForInbound(owner.Id)

	got, err := (&InboundService{}).checkPortConflict(&model.Inbound{
		Tag:      "takes-the-slot",
		Enable:   true,
		Listen:   "0.0.0.0",
		Port:     relayPort,
		Protocol: model.VLESS,
		Settings: `{"clients":[]}`,
	}, 0)
	if err != nil {
		t.Fatalf("checkPortConflict: %v", err)
	}
	if got == nil {
		t.Fatalf("inbound #%d is disabled but still owns relay port %d; the save must be refused",
			owner.Id, relayPort)
	}
	if !strings.Contains(got.String(), owner.Tag) {
		t.Fatalf("the conflict must name the inbound owning the port, got %q", got.String())
	}
}

// A row adopted from a node keeps the protocol it arrived with and its central
// id (inbound_node.go:737), but gets no relay -- so its slot can never be taken.
func TestCheckPortConflict_NodeAssignedAmneziawgOwnsNoRelaySlot(t *testing.T) {
	setupConflictDB(t)
	blocker := addAmneziaWGInbound(t, "awg-blocker", 51820, true)

	nodeID := 7
	adopted := &model.Inbound{
		Tag:      "awg-adopted",
		Enable:   true,
		Listen:   "0.0.0.0",
		Port:     51821,
		Protocol: model.AmneziaWG,
		Settings: awgRelayWindowSettings(t, "awg-adopted"),
		NodeID:   &nodeID,
	}
	collidingID := blocker.Id + 435
	if amneziawgnet.SOCKSPortForInbound(collidingID) != amneziawgnet.SOCKSPortForInbound(blocker.Id) {
		t.Fatalf("fixture: id %d does not derive the blocker's relay port", collidingID)
	}

	got, err := (&InboundService{}).checkPortConflict(adopted, collidingID)
	if err != nil {
		t.Fatalf("checkPortConflict: %v", err)
	}
	if got != nil {
		t.Fatalf("id %d is node-assigned and binds no relay, so it cannot collide; got %q",
			collidingID, got.String())
	}
}
