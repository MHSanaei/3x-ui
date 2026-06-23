package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// mkInboundStream is mkInbound with explicit stream settings, needed to make an
// inbound flow-eligible (VLESS + tcp + reality/tls).
func mkInboundStream(t *testing.T, port int, proto model.Protocol, settings, stream string) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		Tag:            string(proto) + "-stream-" + emailSafe(port),
		Enable:         true,
		Port:           port,
		Protocol:       proto,
		Settings:       settings,
		StreamSettings: stream,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound %d: %v", port, err)
	}
	return ib
}

func emailSafe(port int) string {
	return string(rune('a'+port%26)) + string(rune('a'+(port/26)%26))
}

func flowOf(t *testing.T, svc *ClientService, email string) string {
	t.Helper()
	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail(%q): %v", email, err)
	}
	return rec.Flow
}

const realityStream = `{"network":"tcp","security":"reality"}`
const wsStream = `{"network":"ws","security":"none"}`

// TestBulkAdjust_FlowSetAndClear covers the happy path: a vision flow is applied
// on an eligible VLESS inbound and later cleared with the "none" directive. Both
// transitions are real config changes, so they must request a restart.
func TestBulkAdjust_FlowSetAndClear(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	clients := []model.Client{
		{Email: "f1@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "f1", Enable: true},
		{Email: "f2@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "f2", Enable: true},
	}
	ib := mkInboundStream(t, 30001, model.VLESS, clientsSettings(t, clients), realityStream)
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed: %v", err)
	}
	emails := emailsOf(clients)

	// Set vision flow.
	res, restart, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "xtls-rprx-vision-udp443")
	if err != nil {
		t.Fatalf("BulkAdjust set: %v", err)
	}
	if res.Adjusted != 2 {
		t.Fatalf("expected 2 adjusted, got %d (skipped=%v)", res.Adjusted, res.Skipped)
	}
	if !restart {
		t.Fatalf("setting flow should request a restart")
	}
	for _, e := range emails {
		if got := flowOf(t, svc, e); got != "xtls-rprx-vision-udp443" {
			t.Fatalf("%s flow = %q, want xtls-rprx-vision-udp443", e, got)
		}
	}

	// Setting the same flow again is a no-op: honored (counted) but no restart.
	if _, restart2, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "xtls-rprx-vision-udp443"); err != nil {
		t.Fatalf("BulkAdjust idempotent: %v", err)
	} else if restart2 {
		t.Fatalf("re-setting identical flow should not request a restart")
	}

	// Clear flow.
	cres, crestart, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "none")
	if err != nil {
		t.Fatalf("BulkAdjust clear: %v", err)
	}
	if cres.Adjusted != 2 {
		t.Fatalf("expected 2 cleared, got %d (skipped=%v)", cres.Adjusted, cres.Skipped)
	}
	if !crestart {
		t.Fatalf("clearing flow should request a restart")
	}
	for _, e := range emails {
		if got := flowOf(t, svc, e); got != "" {
			t.Fatalf("%s flow = %q, want empty after clear", e, got)
		}
	}
}

// TestBulkAdjust_FlowIneligibleSkipped verifies a vision flow is refused on an
// inbound that cannot carry it (ws transport), reported as skipped, and the
// client's flow is left untouched.
func TestBulkAdjust_FlowIneligibleSkipped(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	clients := []model.Client{
		{Email: "ws1@x", ID: "33333333-3333-3333-3333-333333333333", SubID: "ws1", Enable: true},
	}
	ib := mkInboundStream(t, 30101, model.VLESS, clientsSettings(t, clients), wsStream)
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, restart, err := svc.BulkAdjust(inboundSvc, []string{"ws1@x"}, 0, 0, "xtls-rprx-vision")
	if err != nil {
		t.Fatalf("BulkAdjust: %v", err)
	}
	if res.Adjusted != 0 {
		t.Fatalf("ineligible inbound should adjust nothing, got %d", res.Adjusted)
	}
	if restart {
		t.Fatalf("no change should not request a restart")
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Email != "ws1@x" {
		t.Fatalf("expected ws1@x in skipped, got %v", res.Skipped)
	}
	if got := flowOf(t, svc, "ws1@x"); got != "" {
		t.Fatalf("flow should stay empty on ineligible inbound, got %q", got)
	}
}

// TestBulkAdjust_NoDirectiveErrors guards the relaxed precondition: with no
// days, traffic, or flow set there is nothing to do.
func TestBulkAdjust_NoDirectiveErrors(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	if _, _, err := svc.BulkAdjust(inboundSvc, []string{"any@x"}, 0, 0, ""); err == nil {
		t.Fatalf("expected error when no adjustment is specified")
	}
	// An unknown flow directive is ignored (treated as ""), so it also errors.
	if _, _, err := svc.BulkAdjust(inboundSvc, []string{"any@x"}, 0, 0, "bogus-flow"); err == nil {
		t.Fatalf("unknown flow should be ignored and error like an empty directive")
	}
}
