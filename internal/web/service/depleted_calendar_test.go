package service

import (
	"strings"
	"testing"
)

// A calendar client has reset = 0, so the old predicate called it depleted at all
// times and the operator's purge deleted it along with its traffic row (#6239).
func TestDepletedClauseExcludesCalendarClients(t *testing.T) {
	if !strings.Contains(depletedClientsClause, "reset_day = 0") {
		t.Fatalf("predicate ignores reset_day, so a calendar client would be purged: %q", depletedClientsClause)
	}
	if !strings.Contains(depletedClientsClause, "reset = 0") {
		t.Fatalf("predicate no longer protects interval clients: %q", depletedClientsClause)
	}
}
