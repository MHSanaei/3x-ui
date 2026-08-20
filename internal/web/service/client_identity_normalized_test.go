package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Identity now comes from the clients table, so settings JSON that drifted from
// it no longer decides who may claim an email (#6252).
func TestAddInboundClientIgnoresStaleSettingsSubIds(t *testing.T) {
	t.Run("stale entry no longer blocks the email", func(t *testing.T) {
		setupBulkDB(t)
		cs := &ClientService{}
		is := &InboundService{}

		target := mkInbound(t, 21101, model.VLESS, `{"clients": []}`)
		// Never synced, so no clients row backs it: pure settings-JSON drift.
		mkInbound(t, 21102, model.VLESS, `{"clients": [{"email": "bob@x", "subId": "s-old", "enable": true}]}`)

		add := []model.Client{{ID: "id-bob", Email: "bob@x", Enable: true, SubID: "s-new"}}
		if _, err := cs.AddInboundClient(is, &model.Inbound{Id: target.Id, Settings: clientsSettings(t, add)}); err != nil {
			t.Fatalf("AddInboundClient rejected by a stale settings entry: %v", err)
		}
		if got := recordSubID(t, "bob@x"); got != "s-new" {
			t.Errorf("stored subId = %q, want %q", got, "s-new")
		}
	})

	t.Run("two drifted subIds no longer lock out the matching one", func(t *testing.T) {
		setupBulkDB(t)
		cs := &ClientService{}
		is := &InboundService{}

		seed := []model.Client{{ID: "id-bob", Email: "bob@x", Enable: true, SubID: "s1"}}
		owner := mkInbound(t, 21103, model.VLESS, clientsSettings(t, seed))
		if err := cs.SyncInbound(nil, owner.Id, seed); err != nil {
			t.Fatalf("seed SyncInbound: %v", err)
		}
		// A second inbound whose JSON disagrees about the subId. The old scan
		// locked the email to "" and then rejected even the correct subId.
		mkInbound(t, 21104, model.VLESS, `{"clients": [{"email": "bob@x", "subId": "s2", "enable": true}]}`)
		target := mkInbound(t, 21105, model.VLESS, `{"clients": []}`)

		add := []model.Client{{ID: "id-bob", Email: "bob@x", Enable: true, SubID: "s1"}}
		if _, err := cs.AddInboundClient(is, &model.Inbound{Id: target.Id, Settings: clientsSettings(t, add)}); err != nil {
			t.Fatalf("AddInboundClient rejected the matching subId: %v", err)
		}
	})
}

// A mismatched subId must still be rejected: the check moved tables, it did not
// get weaker.
func TestAddInboundClientStillRejectsMismatchedSubId(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}
	is := &InboundService{}

	seed := []model.Client{{ID: "id-bob", Email: "bob@x", Enable: true, SubID: "s1"}}
	owner := mkInbound(t, 21111, model.VLESS, clientsSettings(t, seed))
	if err := cs.SyncInbound(nil, owner.Id, seed); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}
	target := mkInbound(t, 21112, model.VLESS, `{"clients": []}`)

	add := []model.Client{{ID: "id-other", Email: "bob@x", Enable: true, SubID: "s-different"}}
	_, err := cs.AddInboundClient(is, &model.Inbound{Id: target.Id, Settings: clientsSettings(t, add)})
	if err == nil {
		t.Fatal("a different subId for a taken email was accepted")
	}
	if !strings.Contains(err.Error(), "Duplicate email") {
		t.Errorf("error = %q, want it to mention Duplicate email", err)
	}
}

// emailsUsedByOtherInbounds keys on lower(email); the clients table stores the
// email as typed under a case-sensitive unique index, so a plain IN would miss.
func TestEmailsUsedByOtherInboundsMatchesCaseInsensitively(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}
	is := &InboundService{}

	seed := []model.Client{{ID: "id-a", Email: "Alice@x", Enable: true, SubID: "s-a"}}
	ibA := mkInbound(t, 21121, model.VLESS, clientsSettings(t, seed))
	ibB := mkInbound(t, 21122, model.VLESS, clientsSettings(t, seed))
	for _, ib := range []*model.Inbound{ibA, ibB} {
		if err := cs.SyncInbound(nil, ib.Id, seed); err != nil {
			t.Fatalf("seed SyncInbound: %v", err)
		}
	}

	shared, err := is.emailsUsedByOtherInbounds([]string{"alice@x"}, ibA.Id)
	if err != nil {
		t.Fatalf("emailsUsedByOtherInbounds: %v", err)
	}
	if !shared["alice@x"] {
		t.Error("lower-cased lookup missed a stored mixed-case email")
	}
	used, err := is.emailUsedByOtherInbounds("alice@x", ibA.Id)
	if err != nil {
		t.Fatalf("emailUsedByOtherInbounds: %v", err)
	}
	if !used {
		t.Error("emailUsedByOtherInbounds missed a stored mixed-case email")
	}

	// The traffic row is shared, so removing the client from one inbound keeps it.
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: ibA.Id, Email: "Alice@x", Enable: true,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	if _, err := cs.DelInboundClientByEmail(is, ibA.Id, "Alice@x", false, false); err != nil {
		t.Fatalf("DelInboundClientByEmail: %v", err)
	}
	var count int64
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", "Alice@x").Count(&count).Error; err != nil {
		t.Fatalf("count traffic: %v", err)
	}
	if count == 0 {
		t.Error("traffic row purged even though the email is still on another inbound")
	}
}

// Guard, not a reproducer: this passes before the delta too. It pins the one
// delta case that is not obviously safe — the rename the taken-email guard
// refuses, where the old record must still lose this inbound's link.
func TestUpdateInboundClientRenameToTakenEmailDetachesOldLink(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}
	is := &InboundService{}

	const subID = "s-shared"
	oldSeed := []model.Client{{ID: "id-old", Email: "old@x", Enable: true, SubID: subID}}
	newSeed := []model.Client{{ID: "id-new", Email: "new@x", Enable: true, SubID: subID}}
	ibX := mkInbound(t, 21131, model.VLESS, clientsSettings(t, oldSeed))
	ibY := mkInbound(t, 21132, model.VLESS, clientsSettings(t, newSeed))
	if err := cs.SyncInbound(nil, ibX.Id, oldSeed); err != nil {
		t.Fatalf("seed X: %v", err)
	}
	if err := cs.SyncInbound(nil, ibY.Id, newSeed); err != nil {
		t.Fatalf("seed Y: %v", err)
	}

	renamed := []model.Client{{ID: "id-old", Email: "new@x", Enable: true, SubID: subID}}
	if _, err := cs.UpdateInboundClient(is,
		&model.Inbound{Id: ibX.Id, Settings: clientsSettings(t, renamed)}, "old@x"); err != nil {
		t.Fatalf("UpdateInboundClient: %v", err)
	}

	links := linksOf(t, ibX.Id)
	if len(links) != 1 {
		t.Fatalf("inbound X link count = %d, want 1: %v", len(links), links)
	}
	if _, ok := links[recordID(t, "new@x")]; !ok {
		t.Error("inbound X is not linked to the new@x record")
	}
	// The refused rename leaves old@x behind; it must not still claim inbound X.
	if _, ok := links[recordID(t, "old@x")]; ok {
		t.Error("old@x kept its link to inbound X after the rename")
	}
}

func recordSubID(t *testing.T, email string) string {
	t.Helper()
	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("record %q: %v", email, err)
	}
	return rec.SubID
}

// The stored record must carry the subId the panel generated into the settings
// JSON. Building the membership delta from the pre-stamp request values instead
// of the stamped wire entries silently desyncs the two.
func TestAddInboundClientPersistsTheGeneratedSubId(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}
	is := &InboundService{}

	ib := mkInbound(t, 21141, model.VLESS, `{"clients": []}`)
	add := []model.Client{{ID: "id-nosub", Email: "nosub@x", Enable: true}}
	if _, err := cs.AddInboundClient(is, &model.Inbound{Id: ib.Id, Settings: clientsSettings(t, add)}); err != nil {
		t.Fatalf("AddInboundClient: %v", err)
	}

	stored := recordSubID(t, "nosub@x")
	if stored == "" {
		t.Fatal("client record has no subId; the generated one was not persisted")
	}
	inSettings := settingsSubID(t, ib.Id, "nosub@x")
	if stored != inSettings {
		t.Errorf("record subId = %q but settings JSON says %q: the two representations desynced",
			stored, inSettings)
	}
}

// clients.email is unique but case-sensitive, so an identity check that does not
// fold case lets a second record for the same address be created.
func TestAddInboundClientRejectsCaseVariantOfTakenEmail(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}
	is := &InboundService{}

	seed := []model.Client{{ID: "id-mix", Email: "Bob@x", Enable: true, SubID: "s-mix"}}
	owner := mkInbound(t, 21151, model.VLESS, clientsSettings(t, seed))
	if err := cs.SyncInbound(nil, owner.Id, seed); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}
	target := mkInbound(t, 21152, model.VLESS, `{"clients": []}`)

	add := []model.Client{{ID: "id-other", Email: "bob@x", Enable: true, SubID: "s-other"}}
	_, err := cs.AddInboundClient(is, &model.Inbound{Id: target.Id, Settings: clientsSettings(t, add)})
	if err == nil {
		var count int64
		database.GetDB().Model(&model.ClientRecord{}).
			Where("LOWER(email) = ?", "bob@x").Count(&count)
		t.Fatalf("a case variant of a taken email was accepted; clients now holds %d rows for bob@x", count)
	}
	if !strings.Contains(err.Error(), "Duplicate email") {
		t.Errorf("error = %q, want it to mention Duplicate email", err)
	}
}

func settingsSubID(t *testing.T, inboundId int, email string) string {
	t.Helper()
	var ib model.Inbound
	if err := database.GetDB().First(&ib, inboundId).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	clients, err := ParseInboundSettingsClients(ib.Settings)
	if err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	for _, c := range clients {
		if c.Email == email {
			return c.SubID
		}
	}
	t.Fatalf("%q not found in settings", email)
	return ""
}
