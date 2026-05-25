package service

// Guard tests for the client-lifecycle methods on InboundService when
// applied to account-based inbounds (Socks/Mixed/HTTP).
//
// The intent here is narrow: prove that AddInboundClient,
// UpdateInboundClient, DelInboundClient and CopyInboundClients refuse
// account-based protocols cleanly *before* they ever hit the
// `settings["clients"].([]any)` cast — which would otherwise panic,
// because account-based inbounds carry settings.accounts[] instead.
//
// We deliberately don't exercise the happy path for client-based
// protocols here — that's covered elsewhere — and we don't need to
// boot xray / runtimes because the guards short-circuit at the very
// top of each method. A tiny in-memory sqlite from setupConflictDB is
// enough.

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// socksAccountsSettings is a minimal but realistic settings payload for
// a SOCKS5 inbound. The shape (accounts[]) is what makes the unguarded
// `settings["clients"].([]any)` cast panic — we keep it real so the
// regression is obvious if the guards ever get removed.
const socksAccountsSettings = `{"auth":"password","accounts":[{"user":"alice","pass":"hunter2"}],"udp":false,"ip":"127.0.0.1"}`

// clientPayloadJSON is a stub "client add" payload. The guards must
// reject the call regardless of what the client envelope contains, so
// the actual fields here don't matter — what matters is that the call
// is made against a SOCKS inbound.
const clientPayloadJSON = `{"clients":[{"id":"00000000-0000-0000-0000-000000000000","email":"ignored@example.com","enable":true}]}`

func seedSocksInbound(t *testing.T) *model.Inbound {
	t.Helper()
	seedInboundConflict(t, "socks-guard", "0.0.0.0", 1080, model.Socks, `{"network":"tcp"}`, socksAccountsSettings)

	var ib model.Inbound
	if err := database.GetDB().Where("tag = ?", "socks-guard").First(&ib).Error; err != nil {
		t.Fatalf("load seeded socks inbound: %v", err)
	}
	return &ib
}

func seedVlessInbound(t *testing.T) *model.Inbound {
	t.Helper()
	const vlessSettings = `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"v@example.com","enable":true}],"decryption":"none"}`
	seedInboundConflict(t, "vless-source", "0.0.0.0", 1443, model.VLESS, `{"network":"tcp"}`, vlessSettings)

	var ib model.Inbound
	if err := database.GetDB().Where("tag = ?", "vless-source").First(&ib).Error; err != nil {
		t.Fatalf("load seeded vless inbound: %v", err)
	}
	return &ib
}

// expectAccountBasedError fails the test unless err is the well-known
// "client lifecycle is not supported for account-based protocol" error
// emitted by the guards. We match on substring to stay decoupled from
// the exact common.NewError formatting (which space-joins its args).
func expectAccountBasedError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected account-based guard error, got nil — guard may be missing")
	}
	msg := err.Error()
	if !strings.Contains(msg, "account-based protocol") {
		t.Fatalf("expected account-based guard error, got: %v", err)
	}
}

func TestAddInboundClient_RejectsSocks(t *testing.T) {
	setupConflictDB(t)
	ib := seedSocksInbound(t)

	svc := &InboundService{}
	_, err := svc.AddInboundClient(&model.Inbound{
		Id:       ib.Id,
		Settings: clientPayloadJSON,
	})
	expectAccountBasedError(t, err)
}

func TestUpdateInboundClient_RejectsSocks(t *testing.T) {
	setupConflictDB(t)
	ib := seedSocksInbound(t)

	svc := &InboundService{}
	_, err := svc.UpdateInboundClient(&model.Inbound{
		Id:       ib.Id,
		Settings: clientPayloadJSON,
	}, "any-client-id")
	expectAccountBasedError(t, err)
}

func TestDelInboundClient_RejectsSocks(t *testing.T) {
	setupConflictDB(t)
	ib := seedSocksInbound(t)

	svc := &InboundService{}
	_, err := svc.DelInboundClient(ib.Id, "any-client-id")
	expectAccountBasedError(t, err)
}

// SOCKS as source and as target both have to be refused — neither
// direction has well-defined semantics (downcasting a rich client to
// {user, pass} would silently drop sub-id / totalGB / expiry; upcasting
// the other way would invent fields that the runtime can't honor).
func TestCopyInboundClients_RejectsSocksSource(t *testing.T) {
	setupConflictDB(t)
	socks := seedSocksInbound(t)
	vless := seedVlessInbound(t)

	svc := &InboundService{}
	_, _, err := svc.CopyInboundClients(vless.Id, socks.Id, nil, "")
	expectAccountBasedError(t, err)
}

func TestCopyInboundClients_RejectsSocksTarget(t *testing.T) {
	setupConflictDB(t)
	socks := seedSocksInbound(t)
	vless := seedVlessInbound(t)

	svc := &InboundService{}
	_, _, err := svc.CopyInboundClients(socks.Id, vless.Id, nil, "")
	expectAccountBasedError(t, err)
}

// Sanity check: the guards must NOT fire on client-based inbounds.
// If this ever flips, AddInboundClient is broken for everyone, not
// just SOCKS users. We don't assert success (the call may legitimately
// fail later in the pipeline because we haven't booted xray) — we just
// assert that whatever error comes back is *not* the guard error.
func TestAddInboundClient_AllowsVless(t *testing.T) {
	setupConflictDB(t)
	ib := seedVlessInbound(t)

	svc := &InboundService{}
	_, err := svc.AddInboundClient(&model.Inbound{
		Id:       ib.Id,
		Settings: clientPayloadJSON,
	})
	if err != nil && strings.Contains(err.Error(), "account-based protocol") {
		t.Fatalf("guard should not fire on VLESS, got: %v", err)
	}
}
