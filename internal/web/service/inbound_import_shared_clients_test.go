package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// makeImportInbound builds an inbound shaped like the import payload: a clients
// JSON blob plus carried-over ClientStats (the exported traffic counters). The
// stats mirror what controller.importInbound feeds AddInbound after zeroing ids.
func makeImportInbound(tag string, port int, settings string, stats []xray.ClientTraffic) *model.Inbound {
	for i := range stats {
		stats[i].Id = 0
		stats[i].Enable = true
	}
	return &model.Inbound{
		UserId:         1,
		Tag:            tag,
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           port,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       settings,
		ClientStats:    stats,
	}
}

// TestAddInbound_ImportTwoInboundsSharingClients reproduces the panel report:
// importing inbound #1 then inbound #2 when both carry the same clients (same
// email + subId) used to fail with "UNIQUE constraint failed: client_traffics.email".
// The shared email already owns a row from the first import, and the second
// inbound's ClientStats association tried to plain-INSERT it again.
func TestAddInbound_ImportTwoInboundsSharingClients(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}

	// Inbound #1: clients alice (shared) and bob (unique to #1).
	settings1 := `{"clients":[` +
		`{"id":"11111111-1111-1111-1111-111111111111","email":"alice","subId":"s-alice","enable":true},` +
		`{"id":"22222222-2222-2222-2222-222222222222","email":"bob","subId":"s-bob","enable":true}` +
		`],"decryption":"none","encryption":"none"}`
	in1 := makeImportInbound("in-9101-tcp", 9101, settings1, []xray.ClientTraffic{
		{Email: "alice", Up: 100, Down: 200, Total: 1000},
		{Email: "bob", Up: 1, Down: 2, Total: 1000},
	})
	if _, _, err := svc.AddInbound(in1); err != nil {
		t.Fatalf("import inbound #1: %v", err)
	}

	// Inbound #2: clients alice (same email+subId as #1) and carol (unique to #2).
	settings2 := `{"clients":[` +
		`{"id":"11111111-1111-1111-1111-111111111111","email":"alice","subId":"s-alice","enable":true},` +
		`{"id":"33333333-3333-3333-3333-333333333333","email":"carol","subId":"s-carol","enable":true}` +
		`],"decryption":"none","encryption":"none"}`
	in2 := makeImportInbound("in-9102-tcp", 9102, settings2, []xray.ClientTraffic{
		{Email: "alice", Up: 999, Down: 999, Total: 9999}, // would clobber the shared row if inserted
		{Email: "carol", Up: 3, Down: 4, Total: 1000},
	})
	if _, _, err := svc.AddInbound(in2); err != nil {
		t.Fatalf("import inbound #2 (the reported failure): %v", err)
	}

	// One traffic row per distinct email — no duplicate "alice".
	for _, tc := range []struct {
		email string
		want  int64
	}{
		{"alice", 100}, // preserved from import #1, not clobbered by #2's 999
		{"bob", 1},
		{"carol", 3},
	} {
		var rows []xray.ClientTraffic
		if err := database.GetDB().Where("email = ?", tc.email).Find(&rows).Error; err != nil {
			t.Fatalf("query %s: %v", tc.email, err)
		}
		if len(rows) != 1 {
			t.Fatalf("email %q: got %d traffic rows, want exactly 1", tc.email, len(rows))
		}
		if rows[0].Up != tc.want {
			t.Fatalf("email %q: Up = %d, want %d (shared row should keep the first import's counters)", tc.email, rows[0].Up, tc.want)
		}
	}
}

// TestAddInbound_ImportStatsMissingClientStillGetsTrafficRow covers an import
// payload whose clientStats doesn't cover every client in settings (older
// exports / hand-edited JSON): the uncovered client must still end up with a
// traffic row, or it would escape quota and expiry accounting.
func TestAddInbound_ImportStatsMissingClientStillGetsTrafficRow(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}

	settings := `{"clients":[` +
		`{"id":"44444444-4444-4444-4444-444444444444","email":"dave","subId":"s-dave","enable":true,"totalGB":1000},` +
		`{"id":"55555555-5555-5555-5555-555555555555","email":"erin","subId":"s-erin","enable":true,"totalGB":2000}` +
		`],"decryption":"none","encryption":"none"}`
	// Stats cover dave only; erin is missing.
	in := makeImportInbound("in-9103-tcp", 9103, settings, []xray.ClientTraffic{
		{Email: "dave", Up: 7, Down: 8, Total: 1000},
	})
	if _, _, err := svc.AddInbound(in); err != nil {
		t.Fatalf("import inbound: %v", err)
	}

	var dave xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "dave").First(&dave).Error; err != nil {
		t.Fatalf("dave row: %v", err)
	}
	if dave.Up != 7 {
		t.Fatalf("dave Up = %d, want 7 (imported counters preserved)", dave.Up)
	}

	var erin xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "erin").First(&erin).Error; err != nil {
		t.Fatalf("erin must still get a traffic row despite missing from clientStats: %v", err)
	}
	if erin.Up != 0 || erin.Down != 0 {
		t.Fatalf("erin counters = %d/%d, want zeroed", erin.Up, erin.Down)
	}
	if erin.Total != 2000 {
		t.Fatalf("erin Total = %d, want 2000 (quota taken from client settings)", erin.Total)
	}
}
