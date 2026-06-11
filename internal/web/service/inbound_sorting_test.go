package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func makeInboundWithSortingIndex(tag string, port int, sortingIndex int16) *model.Inbound {
	return &model.Inbound{
		UserId:         1,
		Tag:            tag,
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           port,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{"clients":[]}`,
		SortingIndex:   sortingIndex,
	}
}

// TestUpdateInbound_PersistsSortingIndex verifies that UpdateInbound copies
// SortingIndex from the incoming update payload to the persisted row.
func TestUpdateInbound_PersistsSortingIndex(t *testing.T) {
	setupConflictDB(t)

	ib := makeInboundWithSortingIndex("in-7001-tcp", 7001, 0)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	update := *ib
	update.SortingIndex = 7

	svc := &InboundService{}
	got, _, err := svc.UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if got.SortingIndex != 7 {
		t.Fatalf("returned SortingIndex = %d, want 7", got.SortingIndex)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.SortingIndex != 7 {
		t.Fatalf("persisted SortingIndex = %d, want 7", reloaded.SortingIndex)
	}
}

// TestUpdateInbound_SortingIndexZeroAllowed verifies that UpdateInbound
// accepts SortingIndex = 0 as an intentional value (not treated as "unset").
func TestUpdateInbound_SortingIndexZeroAllowed(t *testing.T) {
	setupConflictDB(t)

	ib := makeInboundWithSortingIndex("in-7002-tcp", 7002, 5)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	update := *ib
	update.SortingIndex = 0

	svc := &InboundService{}
	got, _, err := svc.UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if got.SortingIndex != 0 {
		t.Fatalf("returned SortingIndex = %d, want 0", got.SortingIndex)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.SortingIndex != 0 {
		t.Fatalf("persisted SortingIndex = %d, want 0", reloaded.SortingIndex)
	}
}

// TestGetInbounds_OrdersBySortingIndexThenId verifies that GetInbounds
// returns inbounds ordered by sorting_index ASC, breaking ties by id ASC.
func TestGetInbounds_OrdersBySortingIndexThenId(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	// Insert in an order that would be wrong if sorting_index is not applied.
	ib10a := makeInboundWithSortingIndex("in-8001-tcp", 8001, 10)
	ib5 := makeInboundWithSortingIndex("in-8002-tcp", 8002, 5)
	ib10b := makeInboundWithSortingIndex("in-8003-tcp", 8003, 10)
	for _, ib := range []*model.Inbound{ib10a, ib5, ib10b} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	svc := &InboundService{}
	got, err := svc.GetInbounds(1)
	if err != nil {
		t.Fatalf("GetInbounds: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 inbounds, got %d", len(got))
	}

	// ib5 (index 5) first; then ib10a < ib10b by id.
	wantTags := []string{"in-8002-tcp", "in-8001-tcp", "in-8003-tcp"}
	for i, ib := range got {
		if ib.Tag != wantTags[i] {
			t.Errorf("got[%d].Tag = %q, want %q", i, ib.Tag, wantTags[i])
		}
	}
}

// TestGetInbounds_NegativeSortingIndexSortsFirst verifies that negative
// sorting indices (meaning "push to top") are ordered before zero and positive.
func TestGetInbounds_NegativeSortingIndexSortsFirst(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	ibPos := makeInboundWithSortingIndex("in-9001-tcp", 9001, 5)
	ibZero := makeInboundWithSortingIndex("in-9002-tcp", 9002, 0)
	ibNeg := makeInboundWithSortingIndex("in-9003-tcp", 9003, -1)
	for _, ib := range []*model.Inbound{ibPos, ibZero, ibNeg} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	svc := &InboundService{}
	got, err := svc.GetInbounds(1)
	if err != nil {
		t.Fatalf("GetInbounds: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 inbounds, got %d", len(got))
	}

	wantTags := []string{"in-9003-tcp", "in-9002-tcp", "in-9001-tcp"}
	for i, ib := range got {
		if ib.Tag != wantTags[i] {
			t.Errorf("got[%d].Tag = %q, want %q", i, ib.Tag, wantTags[i])
		}
	}
}

// TestGetAllInbounds_OrdersBySortingIndex verifies that GetAllInbounds
// returns all inbounds ordered by sorting_index ASC regardless of user.
func TestGetAllInbounds_OrdersBySortingIndex(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	ib30 := makeInboundWithSortingIndex("in-a001-tcp", 10001, 30)
	ib10 := makeInboundWithSortingIndex("in-a002-tcp", 10002, 10)
	ib20 := makeInboundWithSortingIndex("in-a003-tcp", 10003, 20)
	for _, ib := range []*model.Inbound{ib30, ib10, ib20} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	svc := &InboundService{}
	got, err := svc.GetAllInbounds()
	if err != nil {
		t.Fatalf("GetAllInbounds: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 inbounds, got %d", len(got))
	}

	wantTags := []string{"in-a002-tcp", "in-a003-tcp", "in-a001-tcp"}
	for i, ib := range got {
		if ib.Tag != wantTags[i] {
			t.Errorf("got[%d].Tag = %q, want %q", i, ib.Tag, wantTags[i])
		}
	}
}

// TestSearchInbounds_OrdersBySortingIndex verifies that SearchInbounds
// returns matched inbounds ordered by sorting_index ASC and excludes
// inbounds whose remark does not match the query.
func TestSearchInbounds_OrdersBySortingIndex(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	makeWithRemark := func(tag string, port int, remark string, idx int16) *model.Inbound {
		ib := makeInboundWithSortingIndex(tag, port, idx)
		ib.Remark = remark
		return ib
	}
	ib20 := makeWithRemark("in-b001-tcp", 11001, "myservice", 20)
	ib10 := makeWithRemark("in-b002-tcp", 11002, "myservice-backup", 10)
	ibOther := makeWithRemark("in-b003-tcp", 11003, "other", 5)
	for _, ib := range []*model.Inbound{ib20, ib10, ibOther} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	svc := &InboundService{}
	got, err := svc.SearchInbounds("myservice")
	if err != nil {
		t.Fatalf("SearchInbounds: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if got[0].Tag != "in-b002-tcp" || got[1].Tag != "in-b001-tcp" {
		t.Errorf("order = [%q, %q], want [in-b002-tcp, in-b001-tcp]", got[0].Tag, got[1].Tag)
	}
}

// TestGetInboundTags_OrdersBySortingIndex verifies that GetInboundTags
// returns a JSON array of tags in sorting_index ASC order.
func TestGetInboundTags_OrdersBySortingIndex(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	ib2 := makeInboundWithSortingIndex("tag-high", 12001, 2)
	ib0 := makeInboundWithSortingIndex("tag-low", 12002, 0)
	ib1 := makeInboundWithSortingIndex("tag-mid", 12003, 1)
	for _, ib := range []*model.Inbound{ib2, ib0, ib1} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	svc := &InboundService{}
	tagsJSON, err := svc.GetInboundTags()
	if err != nil {
		t.Fatalf("GetInboundTags: %v", err)
	}

	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		t.Fatalf("parse tags JSON %q: %v", tagsJSON, err)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(tags), tags)
	}

	wantTags := []string{"tag-low", "tag-mid", "tag-high"}
	for i, tag := range tags {
		if tag != wantTags[i] {
			t.Errorf("tags[%d] = %q, want %q", i, tag, wantTags[i])
		}
	}
}

// TestGetInboundsByTrafficReset_OrdersBySortingIndex verifies that
// GetInboundsByTrafficReset filters by traffic_reset period and returns
// matching inbounds ordered by sorting_index ASC.
func TestGetInboundsByTrafficReset_OrdersBySortingIndex(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	makeWithReset := func(tag string, port int, reset string, idx int16) *model.Inbound {
		ib := makeInboundWithSortingIndex(tag, port, idx)
		ib.TrafficReset = reset
		return ib
	}
	ib30 := makeWithReset("in-c001-tcp", 13001, "daily", 30)
	ib10 := makeWithReset("in-c002-tcp", 13002, "daily", 10)
	ibMonthly := makeWithReset("in-c003-tcp", 13003, "monthly", 5)
	for _, ib := range []*model.Inbound{ib30, ib10, ibMonthly} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	svc := &InboundService{}
	got, err := svc.GetInboundsByTrafficReset("daily")
	if err != nil {
		t.Fatalf("GetInboundsByTrafficReset: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 daily inbounds, got %d", len(got))
	}
	if got[0].Tag != "in-c002-tcp" || got[1].Tag != "in-c001-tcp" {
		t.Errorf("order = [%q, %q], want [in-c002-tcp, in-c001-tcp]", got[0].Tag, got[1].Tag)
	}
}
