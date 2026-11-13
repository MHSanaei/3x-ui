package job

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestNodeClientTotals_DedupesSharedClientRow(t *testing.T) {
	row := xray.ClientTraffic{Email: "a@x", Up: 100, Down: 200}
	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{
		{Tag: "in-443", ClientStats: []xray.ClientTraffic{row, {Email: "b@x", Up: 1, Down: 2}}},
		{Tag: "in-9452", ClientStats: []xray.ClientTraffic{row}},
		nil,
	}}
	got := nodeClientTotals(snap)
	if len(got) != 2 || got["a@x"] != [2]int64{100, 200} || got["b@x"] != [2]int64{1, 2} {
		t.Fatalf("unexpected totals: %v", got)
	}
}

func TestDiffClientTotals_FirstPollOnlyBaselines(t *testing.T) {
	next, deltas := diffClientTotals(nil, map[string][2]int64{"a@x": {10, 20}}, 1000)
	if len(deltas) != 0 {
		t.Fatalf("first poll must not emit speed, got %v", deltas)
	}
	if next["a@x"] != (inboundSample{up: 10, down: 20, at: 1000}) {
		t.Fatalf("baseline not recorded: %v", next)
	}
}

func TestDiffClientTotals_DeltaAndIdle(t *testing.T) {
	prev := map[string]inboundSample{
		"a@x": {up: 10, down: 20, at: 0},
		"b@x": {up: 5, down: 5, at: 0},
	}
	cur := map[string][2]int64{"a@x": {1010, 5020}, "b@x": {5, 5}}
	next, deltas := diffClientTotals(prev, cur, nodeInboundSpeedWindowMs)
	if len(deltas) != 1 || deltas[0].Email != "a@x" || deltas[0].Up != 1000 || deltas[0].Down != 5000 {
		t.Fatalf("unexpected deltas: %+v", deltas)
	}
	// Idle client keeps its change timestamp so a later jump averages over the gap.
	if next["b@x"].at != 0 {
		t.Fatalf("idle client timestamp advanced: %v", next["b@x"])
	}
}

func TestDiffClientTotals_GapIsAveraged(t *testing.T) {
	prev := map[string]inboundSample{"a@x": {up: 0, down: 0, at: 0}}
	// 4000 bytes moved over two windows -> shown as 2000 per window.
	_, deltas := diffClientTotals(prev, map[string][2]int64{"a@x": {0, 4000}}, 2*nodeInboundSpeedWindowMs)
	if len(deltas) != 1 || deltas[0].Down != 2000 {
		t.Fatalf("gap not averaged: %+v", deltas)
	}
}

func TestDiffClientTotals_ResetRebaselines(t *testing.T) {
	prev := map[string]inboundSample{"a@x": {up: 500, down: 500, at: 0}}
	next, deltas := diffClientTotals(prev, map[string][2]int64{"a@x": {0, 0}}, 7000)
	if len(deltas) != 0 {
		t.Fatalf("reset must not emit speed: %+v", deltas)
	}
	if next["a@x"] != (inboundSample{up: 0, down: 0, at: 7000}) {
		t.Fatalf("reset not rebaselined: %v", next["a@x"])
	}
}

func TestRecordNodeClientSpeed_PerNodeBaselinesAndPrune(t *testing.T) {
	j := NewNodeTrafficSyncJob()
	mk := func(up, down int64) *runtime.TrafficSnapshot {
		return &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{
			{ClientStats: []xray.ClientTraffic{{Email: "a@x", Up: up, Down: down}}},
		}}
	}
	j.recordNodeClientSpeed(1, mk(0, 0))
	j.recordNodeClientSpeed(2, mk(1_000_000, 1_000_000))
	if len(j.tickClientSpeed) != 0 || len(j.prevClientTotals) != 2 {
		t.Fatalf("baselines: tick=%v prev=%v", j.tickClientSpeed, j.prevClientTotals)
	}
	j.pruneClientSpeedNodes([]*model.Node{{Id: 1}})
	if _, ok := j.prevClientTotals[2]; ok || len(j.prevClientTotals) != 1 {
		t.Fatalf("deleted node not pruned: %v", j.prevClientTotals)
	}
}
