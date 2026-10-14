package service

import "testing"

// TestBulkResetTraffic_ReenablesDisabledClient covers the re-enable branch of
// BulkResetTraffic: a disabled client whose traffic is bulk-reset must come back
// enabled with its counters zeroed, in all three enable locations. This is the
// path whose s.Update failure was previously swallowed silently.
func TestBulkResetTraffic_ReenablesDisabledClient(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "reset-reenable@x"
	ib := seedLocalDisabledClient(t, svc, 52010, "", email, 1000, 0, 600, 500)

	affected, err := svc.BulkResetTraffic(inboundSvc, []string{email})
	if err != nil {
		t.Fatalf("BulkResetTraffic: %v", err)
	}
	if affected < 1 {
		t.Fatalf("affected = %d, want >= 1", affected)
	}

	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, true)
	if tr := trafficOf(t, email); tr.Up != 0 || tr.Down != 0 {
		t.Fatalf("%s: traffic after reset up=%d down=%d, want 0/0", email, tr.Up, tr.Down)
	}
}

func TestResetAllTraffics_ReenablesDepletedClientEverywhere(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	depleted := seedLocalDisabledClient(t, svc, 52011, "", "spent-all@x", 1000, 0, 600, 500)
	banned := seedLocalDisabledClient(t, svc, 52012, "", "banned-all@x", 100000, 0, 600, 500)

	if _, err := svc.ResetAllTraffics(); err != nil {
		t.Fatalf("ResetAllTraffics: %v", err)
	}

	assertEnableEverywhere(t, svc, inboundSvc, depleted.Id, "spent-all@x", true)
	if recordEnableOf(t, svc, "banned-all@x") {
		t.Fatal("banned-all@x: an operator-disabled client was switched back on by Reset All Client Traffic")
	}
	if jsonClientEnable(t, inboundSvc, banned.Id, "banned-all@x") {
		t.Fatal("banned-all@x: an operator-disabled client was enabled in the inbound JSON")
	}
}
