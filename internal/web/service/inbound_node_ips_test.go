package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRecordLocalClientIps_RoundTripByGuid(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()
	svc := &InboundService{}

	if err := svc.RecordLocalClientIps("guid-A", map[string][]model.ClientIpEntry{
		"u@x": {{IP: "1.1.1.1", Timestamp: now}, {IP: "2.2.2.2", Timestamp: now - 10}},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	trees, err := svc.GetClientIpsByGuid()
	if err != nil {
		t.Fatalf("byGuid: %v", err)
	}
	got := trees["guid-A"]["u@x"]
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %v", got)
	}
	if got[0].IP != "1.1.1.1" { // newest-first ordering
		t.Fatalf("want newest first, got %v", got)
	}
}

func TestRecordLocalClientIps_MergesAndDropsStale(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()
	svc := &InboundService{}

	if err := svc.RecordLocalClientIps("g", map[string][]model.ClientIpEntry{
		"u@x": {{IP: "keep", Timestamp: now - 60}},
	}); err != nil {
		t.Fatalf("record 1: %v", err)
	}
	// Second scan refreshes keep, adds a stale entry (must be dropped) and a fresh one.
	if err := svc.RecordLocalClientIps("g", map[string][]model.ClientIpEntry{
		"u@x": {{IP: "keep", Timestamp: now}, {IP: "stale", Timestamp: now - 4000}, {IP: "new", Timestamp: now - 5}},
	}); err != nil {
		t.Fatalf("record 2: %v", err)
	}

	trees, _ := svc.GetClientIpsByGuid()
	got := map[string]int64{}
	for _, e := range trees["g"]["u@x"] {
		got[e.IP] = e.Timestamp
	}
	if got["keep"] != now {
		t.Fatalf("keep should refresh to now: %v", got)
	}
	if _, ok := got["stale"]; ok {
		t.Fatalf("stale entry should be dropped: %v", got)
	}
	if got["new"] != now-5 {
		t.Fatalf("new missing: %v", got)
	}
}

func TestUpsertNodeClientIps_EmptyMergeDeletesRow(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()
	db := database.GetDB()
	svc := &InboundService{}

	// Seed an already-stale row, then record another all-stale observation: the
	// merge yields nothing fresh, so the row must be removed (not left lingering).
	staleIps, _ := json.Marshal([]model.ClientIpEntry{{IP: "old", Timestamp: now - 999999}})
	if err := db.Create(&model.NodeClientIp{NodeGuid: "g", Email: "u@x", Ips: string(staleIps)}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := svc.RecordLocalClientIps("g", map[string][]model.ClientIpEntry{
		"u@x": {{IP: "old2", Timestamp: now - 999999}},
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	var count int64
	database.GetDB().Model(&model.NodeClientIp{}).
		Where("node_guid = ? AND email = ?", "g", "u@x").Count(&count)
	if count != 0 {
		t.Fatalf("row should be deleted when merge is empty, got %d", count)
	}
}

func TestGetClientIpNodeAttribution_NewestGuidWins(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()
	svc := &InboundService{}

	// Same IP observed on two panels; the most recent observation attributes it.
	if err := svc.RecordLocalClientIps("gA", map[string][]model.ClientIpEntry{
		"u@x": {{IP: "9.9.9.9", Timestamp: now - 100}},
	}); err != nil {
		t.Fatalf("record gA: %v", err)
	}
	if err := svc.MergeClientIpsByGuid(nil, map[string]map[string][]model.ClientIpEntry{
		"gB": {"u@x": {{IP: "9.9.9.9", Timestamp: now}}},
	}); err != nil {
		t.Fatalf("merge gB: %v", err)
	}

	attr, err := svc.GetClientIpNodeAttribution("u@x")
	if err != nil {
		t.Fatalf("attribution: %v", err)
	}
	if attr["9.9.9.9"] != "gB" {
		t.Fatalf("newest guid should win, got %q", attr["9.9.9.9"])
	}
}

func TestGetClientIpsWithNodes_LabelsNodes(t *testing.T) {
	setupClientIpTestDB(t)
	now := time.Now().Unix()
	db := database.GetDB()
	svc := &InboundService{}

	panelGuid, err := (&SettingService{}).GetPanelGuid()
	if err != nil || panelGuid == "" {
		t.Fatalf("panel guid: %v", err)
	}

	if err := db.Create(&model.Node{Name: "edge-1", Guid: "node-guid", Address: "x", Port: 2053, ApiToken: "t"}).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}

	// Flat display set (what the IP-log lists) holds both IPs.
	flat, _ := json.Marshal([]model.ClientIpEntry{{IP: "1.1.1.1", Timestamp: now}, {IP: "2.2.2.2", Timestamp: now}})
	if err := db.Create(&model.InboundClientIps{ClientEmail: "u@x", Ips: string(flat)}).Error; err != nil {
		t.Fatalf("seed flat ips: %v", err)
	}

	// Attribution: 1.1.1.1 seen locally, 2.2.2.2 seen on the node.
	if err := svc.RecordLocalClientIps(panelGuid, map[string][]model.ClientIpEntry{
		"u@x": {{IP: "1.1.1.1", Timestamp: now}},
	}); err != nil {
		t.Fatalf("record local: %v", err)
	}
	if err := svc.MergeClientIpsByGuid(nil, map[string]map[string][]model.ClientIpEntry{
		"node-guid": {"u@x": {{IP: "2.2.2.2", Timestamp: now}}},
	}); err != nil {
		t.Fatalf("merge node: %v", err)
	}

	infos, err := svc.GetClientIpsWithNodes("u@x")
	if err != nil {
		t.Fatalf("getIpsWithNodes: %v", err)
	}
	byIP := map[string]string{}
	for _, in := range infos {
		byIP[in.IP] = in.Node
	}
	if byIP["1.1.1.1"] != "" {
		t.Fatalf("local IP should have empty node, got %q", byIP["1.1.1.1"])
	}
	if byIP["2.2.2.2"] != "edge-1" {
		t.Fatalf("node IP should be labelled edge-1, got %q", byIP["2.2.2.2"])
	}
}
