package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The cycle has to survive the clients table, not just the settings JSON: an
// ordinary edit rebuilds the client from the record and writes it back (#5497).
func TestClientEditKeepsTheTrafficResetCycle(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	clients := []model.Client{
		{
			Email: "cyc@x", ID: "66666666-6666-6666-6666-666666666666", Enable: true,
			TrafficReset: "monthly", TrafficResetDay: 15,
			ExpiryTime: time.Now().Add(24 * time.Hour).UnixMilli(),
		},
	}
	ib := mkInbound(t, 30301, model.VLESS, clientsSettings(t, clients))
	if err := svc.clientService.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	rec, err := svc.clientService.GetRecordByEmail(nil, "cyc@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.TrafficReset != "monthly" || rec.TrafficResetDay != 15 {
		t.Fatalf("clients row holds %q/%d, want monthly/15", rec.TrafficReset, rec.TrafficResetDay)
	}

	// What the edit dialog does: hydrate the record, change something else, save.
	edited := rec.ToClient()
	edited.Comment = "renamed"
	if _, err := svc.clientService.Update(svc, rec.Id, *edited, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var stored model.Inbound
	if err := db.Where("id = ?", ib.Id).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(stored.Settings), &settings); err != nil {
		t.Fatalf("parse inbound settings: %v", err)
	}
	if len(settings.Clients) != 1 {
		t.Fatalf("inbound holds %d clients, want 1", len(settings.Clients))
	}
	if got := settings.Clients[0]; got.TrafficReset != "monthly" || got.TrafficResetDay != 15 {
		t.Fatalf("inbound settings hold %q/%d after an unrelated edit, want monthly/15: the cycle was silently switched off",
			got.TrafficReset, got.TrafficResetDay)
	}

	rec, err = svc.clientService.GetRecordByEmail(nil, "cyc@x")
	if err != nil {
		t.Fatalf("GetRecordByEmail after edit: %v", err)
	}
	if rec.TrafficReset != "monthly" || rec.TrafficResetDay != 15 {
		t.Fatalf("clients row holds %q/%d after an unrelated edit, want monthly/15", rec.TrafficReset, rec.TrafficResetDay)
	}
}

// An unknown cycle would leave a field that reads as configured while no job
// ever selects the client, so it is rejected instead of coerced.
func TestClientTrafficResetValidation(t *testing.T) {
	for _, tc := range []struct {
		period string
		day    int
		ok     bool
	}{
		{"", 0, true},
		{"never", 1, true},
		{"monthly", 31, true},
		{"fortnightly", 1, false},
		{"monthly", 32, false},
		{"monthly", -1, false},
	} {
		err := validateClientTrafficReset(tc.period, tc.day)
		if tc.ok && err != nil {
			t.Errorf("validateClientTrafficReset(%q, %d) = %v, want accepted", tc.period, tc.day, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("validateClientTrafficReset(%q, %d) accepted, want rejected", tc.period, tc.day)
		}
	}
}
