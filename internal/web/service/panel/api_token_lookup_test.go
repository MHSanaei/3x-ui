package panel

import (
	"fmt"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// maxRowsPerQuery reports the largest number of rows any single SELECT inside
// fn returned, which is how a table scan shows up.
func maxRowsPerQuery(t *testing.T, fn func()) int64 {
	t.Helper()
	db := database.GetDB()
	var max int64
	cb := "test:count_api_token_rows"
	if err := db.Callback().Query().After("gorm:query").Register(cb, func(tx *gorm.DB) {
		if tx.RowsAffected > max {
			max = tx.RowsAffected
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	defer func() {
		if err := db.Callback().Query().Remove(cb); err != nil {
			t.Errorf("remove query callback: %v", err)
		}
	}()
	fn()
	return max
}

// Every authenticated request looks its bearer token up. The lookup must read
// the matching row, not every enabled token.
func TestMatchTokenReadsOnlyTheMatchingTokenRow(t *testing.T) {
	setupAPITokenTestDB(t)
	svc := &ApiTokenService{}

	tokens := make([]string, 0, 25)
	for i := range 25 {
		created, err := svc.Create(fmt.Sprintf("token-%d", i), model.ApiScopeAdmin, 0)
		if err != nil {
			t.Fatalf("create token %d: %v", i, err)
		}
		tokens = append(tokens, created.Token)
	}

	rows := maxRowsPerQuery(t, func() {
		matched, ok := svc.MatchToken(tokens[7])
		if !ok {
			t.Fatal("the created token must match")
		}
		if matched.Name != "token-7" {
			t.Fatalf("matched %q, want token-7", matched.Name)
		}
	})

	if rows > 1 {
		t.Fatalf("the lookup read %d token rows; it must read only the matching one", rows)
	}
}
