package service

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type capturingInboundRuntime struct {
	fakeNodeRuntime
	updatedInbound *model.Inbound
}

func (f *capturingInboundRuntime) UpdateInbound(ctx context.Context, oldInbound, updatedInbound *model.Inbound) error {
	snapshot := *updatedInbound
	f.updatedInbound = &snapshot
	return f.fakeNodeRuntime.UpdateInbound(ctx, oldInbound, updatedInbound)
}

func TestNormalizeRealityShortIDRotation_InitializesSchedule(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	inbound := &model.Inbound{
		Protocol:                       model.VLESS,
		StreamSettings:                 realityRotationStream,
		RealityShortIdsRotationEnabled: true,
		RealityShortIdsRotationDays:    7,
		RealityShortIdsRotationCount:   2,
		RealityShortIdsGraceHours:      24,
	}
	if err := normalizeRealityShortIDRotation(inbound, nil, now); err != nil {
		t.Fatalf("normalizeRealityShortIDRotation: %v", err)
	}
	if inbound.RealityShortIdsActiveCount != 4 {
		t.Fatalf("active count = %d, want 4", inbound.RealityShortIdsActiveCount)
	}
	if want := now.AddDate(0, 0, 7).UnixMilli(); inbound.RealityShortIdsNextRotationTime != want {
		t.Fatalf("next rotation = %d, want %d", inbound.RealityShortIdsNextRotationTime, want)
	}
}

func TestNormalizeRealityShortIDRotation_PreservesStateOnOrdinaryEdit(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	old := &model.Inbound{
		Protocol:                        model.VLESS,
		StreamSettings:                  realityRotationStream,
		RealityShortIdsRotationEnabled:  true,
		RealityShortIdsRotationDays:     7,
		RealityShortIdsRotationCount:    2,
		RealityShortIdsGraceHours:       24,
		RealityShortIdsActiveCount:      4,
		RealityShortIdsRotationCursor:   2,
		RealityShortIdsLastRotationTime: 111,
		RealityShortIdsNextRotationTime: 222,
		RealityShortIdsRetireAt:         333,
	}
	incoming := &model.Inbound{
		Protocol:                       model.VLESS,
		StreamSettings:                 realityRotationStream,
		RealityShortIdsRotationEnabled: true,
		RealityShortIdsRotationDays:    7,
		RealityShortIdsRotationCount:   1,
		RealityShortIdsGraceHours:      12,
	}
	if err := normalizeRealityShortIDRotation(incoming, old, now); err != nil {
		t.Fatalf("normalizeRealityShortIDRotation: %v", err)
	}
	if incoming.RealityShortIdsRotationCursor != 2 ||
		incoming.RealityShortIdsLastRotationTime != 111 ||
		incoming.RealityShortIdsNextRotationTime != 222 ||
		incoming.RealityShortIdsRetireAt != 333 {
		t.Fatalf("server-owned state was reset: %+v", incoming)
	}
}

func TestNormalizeRealityShortIDRotation_ClampsRetirementAfterIntervalShrink(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	old := &model.Inbound{
		Protocol:                       model.VLESS,
		StreamSettings:                 realityRotationStream,
		RealityShortIdsRotationEnabled: true,
		RealityShortIdsRotationDays:    30,
		RealityShortIdsRotationCount:   1,
		RealityShortIdsGraceHours:      700,
		RealityShortIdsActiveCount:     2,
		RealityShortIdsRetireAt:        now.Add(699 * time.Hour).UnixMilli(),
	}
	incoming := &model.Inbound{
		Protocol:                       model.VLESS,
		StreamSettings:                 realityRotationStream,
		RealityShortIdsRotationEnabled: true,
		RealityShortIdsRotationDays:    2,
		RealityShortIdsRotationCount:   1,
		RealityShortIdsGraceHours:      24,
	}

	if err := normalizeRealityShortIDRotation(incoming, old, now); err != nil {
		t.Fatalf("normalizeRealityShortIDRotation: %v", err)
	}
	if want := now.Add(24 * time.Hour).UnixMilli(); incoming.RealityShortIdsRetireAt != want {
		t.Fatalf("retire at = %d, want %d", incoming.RealityShortIdsRetireAt, want)
	}
	if want := now.AddDate(0, 0, 2).UnixMilli(); incoming.RealityShortIdsNextRotationTime != want {
		t.Fatalf("next rotation = %d, want %d", incoming.RealityShortIdsNextRotationTime, want)
	}
}

func TestNormalizeRealityShortIDRotation_ManualIDsResetState(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	old := &model.Inbound{
		Protocol:                        model.VLESS,
		StreamSettings:                  realityRotationStream,
		RealityShortIdsRotationEnabled:  true,
		RealityShortIdsRotationDays:     7,
		RealityShortIdsActiveCount:      4,
		RealityShortIdsRotationCursor:   3,
		RealityShortIdsLastRotationTime: 111,
		RealityShortIdsNextRotationTime: 222,
		RealityShortIdsRetireAt:         333,
	}
	incoming := &model.Inbound{
		Protocol:                       model.VLESS,
		StreamSettings:                 `{"security":"reality","realitySettings":{"shortIds":["1111","2222"]}}`,
		RealityShortIdsRotationEnabled: true,
		RealityShortIdsRotationDays:    7,
		RealityShortIdsGraceHours:      24,
	}
	if err := normalizeRealityShortIDRotation(incoming, old, now); err != nil {
		t.Fatalf("normalizeRealityShortIDRotation: %v", err)
	}
	if incoming.RealityShortIdsActiveCount != 2 || incoming.RealityShortIdsRotationCursor != 0 || incoming.RealityShortIdsRetireAt != 0 {
		t.Fatalf("manual short ID edit did not establish a fresh state: %+v", incoming)
	}
	if want := now.AddDate(0, 0, 7).UnixMilli(); incoming.RealityShortIdsNextRotationTime != want {
		t.Fatalf("next rotation = %d, want %d", incoming.RealityShortIdsNextRotationTime, want)
	}
}

func TestUpdateInbound_PreservesRealityShortIDRotationState(t *testing.T) {
	setupConflictDB(t)
	stored := &model.Inbound{
		UserId:                          1,
		Tag:                             "reality-rotation-update",
		Remark:                          "before",
		Enable:                          true,
		Port:                            44303,
		Protocol:                        model.VLESS,
		Settings:                        `{"clients":[]}`,
		StreamSettings:                  realityRotationStream,
		Sniffing:                        `{}`,
		RealityShortIdsRotationEnabled:  true,
		RealityShortIdsRotationDays:     7,
		RealityShortIdsRotationCount:    2,
		RealityShortIdsGraceHours:       24,
		RealityShortIdsActiveCount:      4,
		RealityShortIdsRotationCursor:   2,
		RealityShortIdsLastRotationTime: 111,
		RealityShortIdsNextRotationTime: 222,
		RealityShortIdsRetireAt:         333,
	}
	if err := database.GetDB().Create(stored).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	incoming := *stored
	incoming.Remark = "after"
	incoming.RealityShortIdsRotationCount = 1
	incoming.RealityShortIdsGraceHours = 12
	// These fields are server-owned; an API payload must not be able to reset
	// them while editing unrelated inbound metadata.
	incoming.RealityShortIdsActiveCount = 0
	incoming.RealityShortIdsRotationCursor = 0
	incoming.RealityShortIdsLastRotationTime = 0
	incoming.RealityShortIdsNextRotationTime = 0
	incoming.RealityShortIdsRetireAt = 0

	if _, _, err := (&InboundService{}).UpdateInbound(&incoming); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	reloaded, err := (&InboundService{}).GetInbound(stored.Id)
	if err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if reloaded.Remark != "after" || reloaded.RealityShortIdsRotationCount != 1 || reloaded.RealityShortIdsGraceHours != 12 {
		t.Fatalf("operator-owned rotation config was not saved: %+v", reloaded)
	}
	if reloaded.RealityShortIdsActiveCount != 4 ||
		reloaded.RealityShortIdsRotationCursor != 2 ||
		reloaded.RealityShortIdsLastRotationTime != 111 ||
		reloaded.RealityShortIdsNextRotationTime != 222 ||
		reloaded.RealityShortIdsRetireAt != 333 {
		t.Fatalf("server-owned rotation state was not preserved: %+v", reloaded)
	}
}

func TestProcessRealityShortIDRotations_RotatesThenRetires(t *testing.T) {
	setupConflictDB(t)
	previousManager := runtime.GetManager()
	manager := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	fake := &fakeNodeRuntime{}
	manager.SetLocalRuntimeOverride(fake)
	runtime.SetManager(manager)
	t.Cleanup(func() { runtime.SetManager(previousManager) })

	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	inbound := &model.Inbound{
		UserId:                          1,
		Tag:                             "reality-rotation-test",
		Enable:                          true,
		Port:                            44301,
		Protocol:                        model.VLESS,
		Settings:                        `{"clients":[]}`,
		StreamSettings:                  realityRotationStream,
		RealityShortIdsRotationEnabled:  true,
		RealityShortIdsRotationDays:     7,
		RealityShortIdsRotationCount:    2,
		RealityShortIdsGraceHours:       24,
		RealityShortIdsActiveCount:      4,
		RealityShortIdsNextRotationTime: now.Add(-time.Minute).UnixMilli(),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	service := &InboundService{}
	result, err := service.ProcessRealityShortIDRotations(now)
	if err != nil {
		t.Fatalf("ProcessRealityShortIDRotations: %v", err)
	}
	if result.Rotated != 1 || result.Retired != 0 || result.NeedRestart {
		t.Fatalf("rotation result = %+v, want one live-applied rotation", result)
	}
	if got := fake.updateInbound.Load(); got != 1 {
		t.Fatalf("runtime UpdateInbound calls = %d, want 1", got)
	}

	rotated, err := service.GetInbound(inbound.Id)
	if err != nil {
		t.Fatalf("reload rotated inbound: %v", err)
	}
	accepted := shortIDsFromStream(t, rotated.StreamSettings)
	if len(accepted) != 6 {
		t.Fatalf("accepted IDs = %v, want 4 active + 2 retiring", accepted)
	}
	if !slices.Equal(accepted[4:], []string{"aa", "bbbb"}) {
		t.Fatalf("retiring suffix = %v, want [aa bbbb]", accepted[4:])
	}
	if rotated.RealityShortIdsRetireAt != now.Add(24*time.Hour).UnixMilli() {
		t.Fatalf("retire at = %d, want 24h grace", rotated.RealityShortIdsRetireAt)
	}

	result, err = service.ProcessRealityShortIDRotations(now.Add(23 * time.Hour))
	if err != nil {
		t.Fatalf("maintenance inside grace: %v", err)
	}
	if result.Rotated != 0 || result.Retired != 0 || fake.updateInbound.Load() != 1 {
		t.Fatalf("maintenance changed inbound inside grace: result=%+v calls=%d", result, fake.updateInbound.Load())
	}

	result, err = service.ProcessRealityShortIDRotations(now.Add(25 * time.Hour))
	if err != nil {
		t.Fatalf("retirement maintenance: %v", err)
	}
	if result.Retired != 1 || result.Rotated != 0 || fake.updateInbound.Load() != 2 {
		t.Fatalf("retirement result=%+v calls=%d, want one retirement and two total applies", result, fake.updateInbound.Load())
	}
	retired, err := service.GetInbound(inbound.Id)
	if err != nil {
		t.Fatalf("reload retired inbound: %v", err)
	}
	if got := shortIDsFromStream(t, retired.StreamSettings); len(got) != 4 || !slices.Equal(got, accepted[:4]) {
		t.Fatalf("IDs after retirement = %v, want active prefix %v", got, accepted[:4])
	}
}

func TestProcessRealityShortIDRotations_BuildsLocalRuntimePayload(t *testing.T) {
	setupConflictDB(t)
	previousManager := runtime.GetManager()
	manager := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	fake := &capturingInboundRuntime{}
	manager.SetLocalRuntimeOverride(fake)
	runtime.SetManager(manager)
	t.Cleanup(func() { runtime.SetManager(previousManager) })

	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	inbound := &model.Inbound{
		UserId:   1,
		Tag:      "reality-rotation-runtime-payload",
		Enable:   true,
		Port:     44305,
		Protocol: model.VLESS,
		Settings: `{"clients":[
			{"id":"11111111-1111-4111-8111-111111111111","email":"active","enable":true},
			{"id":"22222222-2222-4222-8222-222222222222","email":"disabled","enable":true}
		],"decryption":"none"}`,
		StreamSettings:                  realityRotationStream,
		RealityShortIdsRotationEnabled:  true,
		RealityShortIdsRotationDays:     7,
		RealityShortIdsGraceHours:       24,
		RealityShortIdsActiveCount:      4,
		RealityShortIdsNextRotationTime: now.Add(-time.Minute).UnixMilli(),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	traffics := []xray.ClientTraffic{
		{InboundId: inbound.Id, Email: "active", Enable: true},
		{InboundId: inbound.Id, Email: "disabled", Enable: false},
	}
	if err := database.GetDB().Create(&traffics).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}
	fallback := &model.InboundFallback{MasterId: inbound.Id, Dest: "8081"}
	if err := database.GetDB().Create(fallback).Error; err != nil {
		t.Fatalf("create fallback: %v", err)
	}

	result, err := (&InboundService{}).ProcessRealityShortIDRotations(now)
	if err != nil {
		t.Fatalf("ProcessRealityShortIDRotations: %v", err)
	}
	if result.Rotated != 1 || fake.updatedInbound == nil {
		t.Fatalf("rotation result = %+v, payload = %#v", result, fake.updatedInbound)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(fake.updatedInbound.Settings), &settings); err != nil {
		t.Fatalf("decode runtime settings: %v", err)
	}
	clients, ok := settings["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("runtime clients = %#v, want only the enabled client", settings["clients"])
	}
	client, ok := clients[0].(map[string]any)
	if !ok || client["email"] != "active" {
		t.Fatalf("runtime client = %#v, want active", clients[0])
	}
	fallbacks, ok := settings["fallbacks"].([]any)
	if !ok || len(fallbacks) != 1 {
		t.Fatalf("runtime fallbacks = %#v, want one generated fallback", settings["fallbacks"])
	}
}

func TestProcessRealityShortIDRotations_InitializesMissingBaselineWithoutRuntimeUpdate(t *testing.T) {
	setupConflictDB(t)
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	inbound := &model.Inbound{
		UserId:                         1,
		Tag:                            "reality-rotation-baseline",
		Enable:                         true,
		Port:                           44302,
		Protocol:                       model.VLESS,
		Settings:                       `{"clients":[]}`,
		StreamSettings:                 realityRotationStream,
		RealityShortIdsRotationEnabled: true,
		RealityShortIdsRotationDays:    7,
		RealityShortIdsGraceHours:      24,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	result, err := (&InboundService{}).ProcessRealityShortIDRotations(now)
	if err != nil {
		t.Fatalf("ProcessRealityShortIDRotations: %v", err)
	}
	if result.Initialized != 1 || result.Rotated != 0 || result.Retired != 0 || result.NeedRestart {
		t.Fatalf("baseline result = %+v", result)
	}
	stored, err := (&InboundService{}).GetInbound(inbound.Id)
	if err != nil {
		t.Fatalf("reload baseline: %v", err)
	}
	if stored.RealityShortIdsActiveCount != 4 || stored.RealityShortIdsNextRotationTime != now.AddDate(0, 0, 7).UnixMilli() {
		t.Fatalf("stored baseline is incorrect: %+v", stored)
	}
}

func TestProcessRealityShortIDRotations_DispatchesToNodeRuntime(t *testing.T) {
	setupConflictDB(t)
	nodeID, fake := setupNodeRuntime(t)
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	inbound := &model.Inbound{
		UserId:                          1,
		NodeID:                          &nodeID,
		Tag:                             "reality-rotation-node",
		Enable:                          true,
		Port:                            44304,
		Protocol:                        model.VLESS,
		Settings:                        `{"clients":[]}`,
		StreamSettings:                  realityRotationStream,
		RealityShortIdsRotationEnabled:  true,
		RealityShortIdsRotationDays:     7,
		RealityShortIdsGraceHours:       24,
		RealityShortIdsActiveCount:      4,
		RealityShortIdsNextRotationTime: now.Add(-time.Minute).UnixMilli(),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}

	result, err := (&InboundService{}).ProcessRealityShortIDRotations(now)
	if err != nil {
		t.Fatalf("ProcessRealityShortIDRotations: %v", err)
	}
	if result.Rotated != 1 || result.NeedRestart {
		t.Fatalf("node rotation result = %+v, want one remote rotation without local restart", result)
	}
	if got := fake.updateInbound.Load(); got != 1 {
		t.Fatalf("node runtime UpdateInbound calls = %d, want 1", got)
	}
	if _, _, dirty, _, err := (&NodeService{}).NodeSyncState(nodeID); err != nil {
		t.Fatalf("NodeSyncState: %v", err)
	} else if !dirty {
		t.Fatal("node rotation must mark config dirty for durable reconciliation")
	}
}
