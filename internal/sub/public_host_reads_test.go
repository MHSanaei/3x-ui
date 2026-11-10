package sub

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

func countSettingReads(t *testing.T) *atomic.Int32 {
	t.Helper()
	var reads atomic.Int32
	db := database.GetDB()
	const hook = "test:count_setting_reads"
	if err := db.Callback().Query().After("gorm:query").Register(hook, func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			reads.Add(1)
		}
	}); err != nil {
		t.Fatalf("register query hook: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(hook) })
	return &reads
}

func seedWildcardListenInbound(t *testing.T, subId, tag string, port int) {
	t.Helper()
	ib := seedSubInbound(t, subId, tag, port, 1, `{"network":"tcp","security":"none"}`)
	if err := database.GetDB().Model(ib).Update("listen", "").Error; err != nil {
		t.Fatalf("clear listen of %s: %v", tag, err)
	}
}

// An inbound with no address of its own falls back to the configured public host.
// That fallback re-read subDomain from the DB for every link in the response.
func TestSubSettingReadsDoNotGrowWithInboundCount(t *testing.T) {
	seedSubDB(t)
	if err := database.GetDB().Save(&model.Setting{Key: "subDomain", Value: "sub.example.com"}).Error; err != nil {
		t.Fatalf("save subDomain: %v", err)
	}
	seedWildcardListenInbound(t, "one", "solo", 4501)
	for i, tag := range []string{"a", "b", "c"} {
		seedWildcardListenInbound(t, "three", tag, 4510+i)
	}
	reads := countSettingReads(t)
	svc := NewSubService("")

	readsFor := func(subId string, wantLinks int) int32 {
		t.Helper()
		before := reads.Load()
		links, _, _, _, err := svc.GetSubs(subId, "198.51.100.7")
		if err != nil {
			t.Fatalf("GetSubs(%s): %v", subId, err)
		}
		if len(links) != wantLinks {
			t.Fatalf("GetSubs(%s) returned %d links, want %d", subId, len(links), wantLinks)
		}
		for _, link := range links {
			if !strings.Contains(link, "@sub.example.com:") {
				t.Fatalf("link should advertise the configured public host: %s", link)
			}
		}
		return reads.Load() - before
	}

	one := readsFor("one", 1)
	three := readsFor("three", 3)
	if three != one {
		t.Fatalf("settings reads: %d for three inbounds, %d for one; the count must not scale with inbounds", three, one)
	}
}
