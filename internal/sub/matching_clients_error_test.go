package sub

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A failed client read must reach the caller: dropping the clients table made
// matchingClients return no members, which read as "this inbound has no
// clients" and produced a 200 subscription that silently lost servers.
func TestMatchingClientsPropagatesReadError(t *testing.T) {
	initSubDB(t)
	ib := seedSubInbound(t, "sub-read-error", "read-error", 4521, 1, "{\"network\":\"tcp\",\"security\":\"none\"}")

	if err := database.GetDB().Migrator().DropTable(&model.ClientRecord{}); err != nil {
		t.Fatalf("drop clients table: %v", err)
	}

	s := &SubService{}
	if _, err := s.matchingClients(ib, "sub-read-error"); err == nil {
		t.Fatal("matchingClients swallowed a client read error")
	}
}
