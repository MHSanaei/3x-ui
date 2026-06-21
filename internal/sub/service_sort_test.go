package sub

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestGetSubs_OrdersBySubSortIndexThenId verifies that subscription output
// lists inbound links ordered by sub_sort_index ASC, breaking ties by id ASC.
// The same query feeds the raw body, the HTML sub page, and the JSON/Clash
// formats, so asserting on GetSubs covers all of them.
func TestGetSubs_OrdersBySubSortIndexThenId(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const subId = "sub-sort"
	db := database.GetDB()

	seed := []struct {
		tag          string
		port         int
		subSortIndex int
		email        string
		uuid         string
	}{
		// Created in this order on purpose: without the ORDER BY the links
		// would come out s3, s1, s2a, s2b (creation order).
		{"sort-3", 42101, 3, "s3@example.com", "0d68a695-4be1-4d92-a9c3-8c0f1c2cf001"},
		{"sort-1", 42102, 1, "s1@example.com", "0d68a695-4be1-4d92-a9c3-8c0f1c2cf002"},
		{"sort-2a", 42103, 2, "s2a@example.com", "0d68a695-4be1-4d92-a9c3-8c0f1c2cf003"},
		{"sort-2b", 42104, 2, "s2b@example.com", "0d68a695-4be1-4d92-a9c3-8c0f1c2cf004"},
	}
	for _, s := range seed {
		settings := fmt.Sprintf(`{"clients": [{"id": %q, "email": %q, "subId": %q, "enable": true}]}`, s.uuid, s.email, subId)
		ib := &model.Inbound{
			UserId:         1,
			Tag:            s.tag,
			Enable:         true,
			Port:           s.port,
			Protocol:       model.VLESS,
			Settings:       settings,
			StreamSettings: `{"network": "tcp", "security": "none"}`,
			SubSortIndex:   s.subSortIndex,
		}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("seed inbound %s: %v", s.tag, err)
		}
		client := &model.ClientRecord{Email: s.email, SubID: subId, UUID: s.uuid, Enable: true}
		if err := db.Create(client).Error; err != nil {
			t.Fatalf("seed client %s: %v", s.email, err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("seed client_inbound %s: %v", s.email, err)
		}
	}

	s := NewSubService("")
	links, emails, _, _, err := s.GetSubs(subId, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != len(seed) {
		t.Fatalf("links = %d, want %d", len(links), len(seed))
	}
	want := []string{"s1@example.com", "s2a@example.com", "s2b@example.com", "s3@example.com"}
	for i, email := range want {
		if emails[i] != email {
			t.Fatalf("emails order = %v, want %v (sub_sort_index ASC, id ASC)", emails, want)
		}
	}
}
