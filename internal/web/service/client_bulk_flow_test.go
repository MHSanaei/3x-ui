package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
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

// TestBulkAdjust_DaysApplyDespiteIneligibleFlow is the regression for the review
// blocker: when a client on a flow-ineligible inbound is adjusted with BOTH a
// days/traffic delta AND a flow directive, the days/traffic change must still be
// persisted to ClientTraffic (not just the inbound JSON / ClientRecord) and the
// client must count as adjusted, while the unhonored flow is reported separately.
func TestBulkAdjust_DaysApplyDespiteIneligibleFlow(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const day = int64(24 * 60 * 60 * 1000)
	const gb = int64(1) << 30
	baseExpiry := time.Now().UnixMilli() + 30*day
	baseTotal := 10 * gb

	clients := []model.Client{
		{Email: "mix@x", ID: "44444444-4444-4444-4444-444444444444", SubID: "mix", Enable: true, ExpiryTime: baseExpiry, TotalGB: baseTotal},
	}
	ib := mkInboundStream(t, 30201, model.VLESS, clientsSettings(t, clients), wsStream)
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// ClientTraffic is the store the enforcement job reads; seed it to match.
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: "mix@x", Enable: true, ExpiryTime: baseExpiry, Total: baseTotal}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	res, _, err := svc.BulkAdjust(inboundSvc, []string{"mix@x"}, 7, gb, "xtls-rprx-vision")
	if err != nil {
		t.Fatalf("BulkAdjust: %v", err)
	}
	if res.Adjusted != 1 {
		t.Fatalf("days/traffic should still be applied: Adjusted=%d skipped=%v", res.Adjusted, res.Skipped)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Email != "mix@x" {
		t.Fatalf("expected mix@x reported for the unhonored flow, got %v", res.Skipped)
	}

	wantExpiry := baseExpiry + 7*day
	wantTotal := baseTotal + gb

	// ClientRecord (inbound-derived) advanced.
	if rec, err := svc.GetRecordByEmail(nil, "mix@x"); err != nil {
		t.Fatalf("record: %v", err)
	} else if rec.ExpiryTime != wantExpiry || rec.TotalGB != wantTotal {
		t.Fatalf("ClientRecord not advanced: expiry=%d total=%d", rec.ExpiryTime, rec.TotalGB)
	}

	// ClientTraffic advanced in lockstep — no divergence.
	var ct xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "mix@x").First(&ct).Error; err != nil {
		t.Fatalf("traffic row: %v", err)
	}
	if ct.ExpiryTime != wantExpiry || ct.Total != wantTotal {
		t.Fatalf("ClientTraffic diverged: expiry=%d total=%d, want expiry=%d total=%d", ct.ExpiryTime, ct.Total, wantExpiry, wantTotal)
	}

	// Flow left untouched on the ineligible inbound.
	if got := flowOf(t, svc, "mix@x"); got != "" {
		t.Fatalf("flow should stay empty on ineligible inbound, got %q", got)
	}
}
