package panel

import (
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func countPanelQueries(t *testing.T, table string, fn func()) int {
	t.Helper()
	db := database.GetDB()
	var n int64
	cb := "test:panel_cache_" + table + "_" + strings.ReplaceAll(t.Name(), "/", "_")
	if err := db.Callback().Query().Before("gorm:query").Register(cb, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == table {
			atomic.AddInt64(&n, 1)
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	defer func() { _ = db.Callback().Query().Remove(cb) }()
	fn()
	return int(atomic.LoadInt64(&n))
}

func TestMatchTokenPositiveOnlyCacheAndInvalidation(t *testing.T) {
	invalidateApiTokenCache()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	svc := &ApiTokenService{}
	created, err := svc.Create("api-client-token", model.ApiScopeAdmin, 0)
	if err != nil {
		t.Fatalf("Create token: %v", err)
	}
	plain := created.Token

	if _, ok := svc.MatchToken(plain); !ok {
		t.Fatal("first MatchToken = false, want true")
	}
	hits := countPanelQueries(t, "api_tokens", func() {
		for range 4 {
			if tok, ok := svc.MatchToken(plain); !ok || tok == nil || tok.Id != created.Id {
				t.Fatalf("cached MatchToken failed: ok=%v", ok)
			}
		}
	})
	if hits != 0 {
		t.Fatalf("cached positive MatchToken issued %d DB queries, want 0", hits)
	}

	// Wrong token must never be negative-cached.
	misses := countPanelQueries(t, "api_tokens", func() {
		for range 2 {
			if _, ok := svc.MatchToken("wrong-secret"); ok {
				t.Fatal("MatchToken accepted wrong token")
			}
		}
	})
	if misses != 2 {
		t.Fatalf("wrong token issued %d DB queries, want 2", misses)
	}

	if err := svc.SetEnabled(created.Id, false); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	if _, ok := svc.MatchToken(plain); ok {
		t.Fatal("MatchToken succeeded after SetEnabled(false)")
	}

	// Re-enabling via direct GORM write must invalidate via write hook too.
	if err := database.GetDB().Model(&model.ApiToken{}).Where("id = ?", created.Id).Update("enabled", true).Error; err != nil {
		t.Fatalf("direct enable update: %v", err)
	}
	if _, ok := svc.MatchToken(plain); !ok {
		t.Fatal("MatchToken = false after direct GORM re-enable")
	}
}

func TestUserGetFirstUserCachedReturnsShallowCopyAndInvalidates(t *testing.T) {
	invalidateFirstUserCache()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	svc := &UserService{}
	first, err := svc.GetFirstUserCached()
	if err != nil {
		t.Fatalf("GetFirstUserCached: %v", err)
	}
	first.Username = "mutated-by-caller"

	secondReads := countPanelQueries(t, "users", func() {
		u, err := svc.GetFirstUserCached()
		if err != nil {
			t.Fatalf("cached GetFirstUserCached: %v", err)
		}
		if u.Username == "mutated-by-caller" {
			t.Fatal("caller mutation poisoned cached User struct")
		}
	})
	if secondReads != 0 {
		t.Fatalf("cached GetFirstUserCached issued %d DB queries, want 0", secondReads)
	}

	if err := svc.UpdateFirstUser("newadmin", "NewPass#123"); err != nil {
		t.Fatalf("UpdateFirstUser: %v", err)
	}
	after, err := svc.GetFirstUserCached()
	if err != nil {
		t.Fatalf("GetFirstUserCached after update: %v", err)
	}
	if after.Username != "newadmin" {
		t.Fatalf("cached username after update = %q, want %q", after.Username, "newadmin")
	}
}
