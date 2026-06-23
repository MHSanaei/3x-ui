package job

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A client attached to several inbounds must be force-disconnected on ALL of
// them when it trips the IP limit, not just the lowest-id one getInboundByEmail
// resolves: Xray meters each (client, inbound) pair under its own per-attachment
// identity, so a single-inbound remove/re-add leaves the over-limit connection
// alive on the client's other inbounds. getInboundsByEmail backs that fix.
func TestGetInboundsByEmail_ReturnsEveryAttachment(t *testing.T) {
	setupIntegrationDB(t)
	db := database.GetDB()

	mk := func(tag string, port int, email string) {
		settings, _ := json.Marshal(map[string]any{
			"clients": []map[string]any{{"email": email, "enable": true}},
		})
		ib := &model.Inbound{Tag: tag, Enable: true, Protocol: model.VLESS, Port: port, Settings: string(settings)}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("seed inbound %s: %v", tag, err)
		}
	}
	mk("in-a", 5001, "shared@x")
	mk("in-b", 5002, "shared@x")
	mk("in-c", 5003, "other@x")

	j := &CheckClientIpJob{}

	all, err := j.getInboundsByEmail("shared@x")
	if err != nil {
		t.Fatalf("getInboundsByEmail: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("getInboundsByEmail(shared@x) = %d inbounds, want 2 (every attachment)", len(all))
	}

	// The single-inbound resolver returns exactly one — which is the gap the
	// across-inbounds disconnect closes.
	one, err := j.getInboundByEmail("shared@x")
	if err != nil {
		t.Fatalf("getInboundByEmail: %v", err)
	}
	if one == nil {
		t.Fatal("getInboundByEmail returned nil for an attached client")
	}
}
