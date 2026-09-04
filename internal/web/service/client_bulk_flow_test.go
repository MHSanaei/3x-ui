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

const (
	realityStream = `{"network":"tcp","security":"reality"}`
	wsStream      = `{"network":"ws","security":"none"}`
)

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
	res, restart, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "xtls-rprx-vision-udp443", nil, "")
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
	if _, restart2, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "xtls-rprx-vision-udp443", nil, ""); err != nil {
		t.Fatalf("BulkAdjust idempotent: %v", err)
	} else if restart2 {
		t.Fatalf("re-setting identical flow should not request a restart")
	}

	// Clear flow.
	cres, crestart, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "none", nil, "")
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

	res, restart, err := svc.BulkAdjust(inboundSvc, []string{"ws1@x"}, 0, 0, "xtls-rprx-vision", nil, "")
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

	if _, _, err := svc.BulkAdjust(inboundSvc, []string{"any@x"}, 0, 0, "", nil, ""); err == nil {
		t.Fatalf("expected error when no adjustment is specified")
	}
	// An unknown flow directive is ignored (treated as ""), so it also errors.
	if _, _, err := svc.BulkAdjust(inboundSvc, []string{"any@x"}, 0, 0, "bogus-flow", nil, ""); err == nil {
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

	res, _, err := svc.BulkAdjust(inboundSvc, []string{"mix@x"}, 7, gb, "xtls-rprx-vision", nil, "")
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

// TestBulkAdjust_HwidLimit verifies setting and clearing HWID limit in bulk.
func TestBulkAdjust_HwidLimit(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	clients := []model.Client{
		{Email: "h1@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sub-h1", Enable: true},
		{Email: "h2@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "sub-h2", Enable: true},
	}
	ib := mkInbound(t, 30301, model.VLESS, clientsSettings(t, clients))
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed: %v", err)
	}
	emails := emailsOf(clients)

	limit2 := 2
	res, restart, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "", &limit2, "")
	if err != nil {
		t.Fatalf("BulkAdjust hwid: %v", err)
	}
	if res.Adjusted != 2 {
		t.Fatalf("expected 2 adjusted, got %d", res.Adjusted)
	}
	if restart {
		t.Fatalf("hwid adjustment should not request xray restart")
	}
	for _, e := range emails {
		rec, rErr := svc.GetRecordByEmail(nil, e)
		if rErr != nil || rec.LimitHwid != 2 {
			t.Fatalf("%s limitHwid = %d (err=%v), want 2", e, rec.LimitHwid, rErr)
		}
	}

	// Reset to 0 (unlimited)
	limit0 := 0
	res0, _, err0 := svc.BulkAdjust(inboundSvc, emails, 0, 0, "", &limit0, "")
	if err0 != nil || res0.Adjusted != 2 {
		t.Fatalf("BulkAdjust hwid 0: err=%v, res=%+v", err0, res0)
	}
	for _, e := range emails {
		rec, _ := svc.GetRecordByEmail(nil, e)
		if rec.LimitHwid != 0 {
			t.Fatalf("%s limitHwid = %d, want 0", e, rec.LimitHwid)
		}
	}
}

// TestBulkAdjust_MtprotoAdTagSetAndClear verifies ad-tag bulk update and clearing.
func TestBulkAdjust_MtprotoAdTagSetAndClear(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const tag1 = "0123456789abcdef0123456789abcdef"
	clients := []model.Client{
		{Email: "tg1@x", Secret: "ee00112233445566778899aabbccddeeff6578616d706c652e636f6d", Enable: true},
		{Email: "tg2@x", Secret: "ee101112131415161718191a1b1c1d1e1f6578616d706c652e636f6d", Enable: true},
	}
	ib := &model.Inbound{
		Tag:      "mtproto-bulk-test",
		Enable:   true,
		Port:     30401,
		Protocol: model.MTProto,
		Settings: clientsSettings(t, clients),
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create mtproto inbound: %v", err)
	}
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed mtproto: %v", err)
	}
	emails := emailsOf(clients)

	// Set ad-tag
	res, restart, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "", nil, tag1)
	if err != nil {
		t.Fatalf("BulkAdjust adTag: %v", err)
	}
	if res.Adjusted != 2 {
		t.Fatalf("expected 2 adjusted, got %d", res.Adjusted)
	}
	if restart {
		t.Fatalf("mtproto adTag update should not request xray restart")
	}
	for _, e := range emails {
		rec, _ := svc.GetRecordByEmail(nil, e)
		if rec.AdTag != tag1 {
			t.Fatalf("%s adTag = %q, want %q", e, rec.AdTag, tag1)
		}
	}

	// Clear ad-tag with "none"
	cres, _, cerr := svc.BulkAdjust(inboundSvc, emails, 0, 0, "", nil, "none")
	if cerr != nil || cres.Adjusted != 2 {
		t.Fatalf("BulkAdjust clear adTag: err=%v, res=%+v", cerr, cres)
	}
	for _, e := range emails {
		rec, _ := svc.GetRecordByEmail(nil, e)
		if rec.AdTag != "" {
			t.Fatalf("%s adTag = %q, want empty after clear", e, rec.AdTag)
		}
	}

	// Invalid ad-tag errors
	if _, _, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "", nil, "invalid-hex"); err == nil {
		t.Fatalf("expected error for invalid hex ad tag")
	}
}

// TestBulkAdjust_AdTagIneligibleSkipped verifies that non-MTProto clients are
// refused adTag adjustment, reported as skipped, and their ClientRecord is untouched.
func TestBulkAdjust_AdTagIneligibleSkipped(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	clients := []model.Client{
		{Email: "vless-notg@x", ID: "55555555-5555-5555-5555-555555555555", SubID: "vless-notg", Enable: true},
	}
	ib := mkInbound(t, 30501, model.VLESS, clientsSettings(t, clients))
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const tag1 = "0123456789abcdef0123456789abcdef"
	res, restart, err := svc.BulkAdjust(inboundSvc, []string{"vless-notg@x"}, 0, 0, "", nil, tag1)
	if err != nil {
		t.Fatalf("BulkAdjust: %v", err)
	}
	if res.Adjusted != 0 {
		t.Fatalf("ineligible protocol should adjust nothing, got %d", res.Adjusted)
	}
	if restart {
		t.Fatalf("no change should not request restart")
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Email != "vless-notg@x" || res.Skipped[0].Reason != "adTag not supported on inbound" {
		t.Fatalf("expected vless-notg@x in skipped with 'adTag not supported on inbound', got %+v", res.Skipped)
	}
	rec, err := svc.GetRecordByEmail(nil, "vless-notg@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.AdTag != "" {
		t.Fatalf("adTag on non-MTProto record should stay empty, got %q", rec.AdTag)
	}
}

// TestBulkAdjust_DaysApplyDespiteIneligibleAdTag verifies that when a non-MTProto
// client is adjusted with both days and adTag, days are applied but adTag is not
// written to ClientRecord and is reported as skipped.
func TestBulkAdjust_DaysApplyDespiteIneligibleAdTag(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const day = int64(24 * 60 * 60 * 1000)
	baseExpiry := time.Now().UnixMilli() + 30*day

	clients := []model.Client{
		{Email: "vless-days@x", ID: "66666666-6666-6666-6666-666666666666", SubID: "vless-days", Enable: true, ExpiryTime: baseExpiry},
	}
	ib := mkInbound(t, 30601, model.VLESS, clientsSettings(t, clients))
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: "vless-days@x", Enable: true, ExpiryTime: baseExpiry}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	const tag1 = "0123456789abcdef0123456789abcdef"
	res, _, err := svc.BulkAdjust(inboundSvc, []string{"vless-days@x"}, 7, 0, "", nil, tag1)
	if err != nil {
		t.Fatalf("BulkAdjust: %v", err)
	}
	if res.Adjusted != 1 {
		t.Fatalf("days should still be applied: Adjusted=%d skipped=%v", res.Adjusted, res.Skipped)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Email != "vless-days@x" || res.Skipped[0].Reason != "adTag not supported on inbound" {
		t.Fatalf("expected vless-days@x reported for unhonored adTag, got %v", res.Skipped)
	}

	rec, err := svc.GetRecordByEmail(nil, "vless-days@x")
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if rec.ExpiryTime != baseExpiry+7*day {
		t.Fatalf("expiry time not advanced: got %d, want %d", rec.ExpiryTime, baseExpiry+7*day)
	}
	if rec.AdTag != "" {
		t.Fatalf("adTag should remain empty on ClientRecord for non-MTProto, got %q", rec.AdTag)
	}
}

// TestBulkAdjust_MixedMtprotoAndVless_AdTag verifies bulk adjust over a mixed
// MTProto and VLESS selection.
func TestBulkAdjust_MixedMtprotoAndVless_AdTag(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const tag1 = "0123456789abcdef0123456789abcdef"
	tgClients := []model.Client{
		{Email: "tg-mix@x", Secret: "ee00112233445566778899aabbccddeeff6578616d706c652e636f6d", Enable: true},
	}
	tgIb := &model.Inbound{
		Tag:      "mtproto-mix",
		Enable:   true,
		Port:     30701,
		Protocol: model.MTProto,
		Settings: clientsSettings(t, tgClients),
	}
	if err := database.GetDB().Create(tgIb).Error; err != nil {
		t.Fatalf("create mtproto: %v", err)
	}
	if err := svc.SyncInbound(nil, tgIb.Id, tgClients); err != nil {
		t.Fatalf("sync mtproto: %v", err)
	}

	vlessClients := []model.Client{
		{Email: "vless-mix@x", ID: "77777777-7777-7777-7777-777777777777", SubID: "vless-mix", Enable: true},
	}
	vlessIb := mkInbound(t, 30702, model.VLESS, clientsSettings(t, vlessClients))
	if err := svc.SyncInbound(nil, vlessIb.Id, vlessClients); err != nil {
		t.Fatalf("sync vless: %v", err)
	}

	emails := []string{"tg-mix@x", "vless-mix@x"}
	res, restart, err := svc.BulkAdjust(inboundSvc, emails, 0, 0, "", nil, tag1)
	if err != nil {
		t.Fatalf("BulkAdjust: %v", err)
	}
	if res.Adjusted != 1 {
		t.Fatalf("expected 1 adjusted (MTProto only), got %d", res.Adjusted)
	}
	if restart {
		t.Fatalf("adTag should not restart xray")
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Email != "vless-mix@x" || res.Skipped[0].Reason != "adTag not supported on inbound" {
		t.Fatalf("expected vless-mix@x in skipped, got %+v", res.Skipped)
	}

	tgRec, _ := svc.GetRecordByEmail(nil, "tg-mix@x")
	if tgRec.AdTag != tag1 {
		t.Fatalf("tg-mix@x adTag = %q, want %q", tgRec.AdTag, tag1)
	}
	vlessRec, _ := svc.GetRecordByEmail(nil, "vless-mix@x")
	if vlessRec.AdTag != "" {
		t.Fatalf("vless-mix@x adTag = %q, want empty", vlessRec.AdTag)
	}
}
