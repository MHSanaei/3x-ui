package sub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// budgetRepeats copies of budgetRemark are well past maxRenderedBytes.
var budgetRemark = strings.Repeat("r", 100_000)

const budgetRepeats = 200

func renderedSubscriptionSizes(t *testing.T, subId string) map[string]int {
	t.Helper()
	base := NewSubService("")
	links, _, _, _, err := base.GetSubs(subId, "req.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	jsonBody, _, err := NewSubJsonService("", "", "", "", base).GetJson(subId, "req.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	clashBody, _, err := NewSubClashService(false, "", false, base).GetClash(subId, "req.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	return map[string]int{"raw": len(strings.Join(links, "\n")), "json": len(jsonBody), "clash": len(clashBody)}
}

// The final documents indent the booked JSON configs again and list every Clash
// proxy name a second time in the PROXY group, hence the factor of two.
func assertBoundedSubscription(t *testing.T, subId string) {
	t.Helper()
	for format, size := range renderedSubscriptionSizes(t, subId) {
		if size < len(budgetRemark) || size > 2*maxRenderedBytes {
			t.Errorf("%s subscription is %d bytes, want %d..%d", format, size, len(budgetRemark), 2*maxRenderedBytes)
		}
	}
}

// Clients sharing a subId repeat the remark just like endpoints do, each in a
// fan-out of its own, so the response as a whole needs the bound too.
func TestSubscriptionBoundsRenderedSizeAcrossClients(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	ib := seedSubInbound(t, "s1", "many", 4431, 1, `{"network":"tcp","security":"none"}`)
	var clients strings.Builder
	for i := range budgetRepeats {
		uuid, email := fmt.Sprintf("11111111-2222-4333-8444-%012d", i), fmt.Sprintf("c%d@e", i)
		fmt.Fprintf(&clients, `{"id":%q,"email":%q,"subId":"s1","enable":true},`, uuid, email)
		record := &model.ClientRecord{Email: email, SubID: "s1", UUID: uuid, Enable: true}
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed client %s: %v", email, err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: record.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("seed client_inbound %s: %v", email, err)
		}
	}
	if err := db.Model(ib).Updates(map[string]any{
		"remark":   budgetRemark,
		"settings": `{"clients":[` + strings.TrimSuffix(clients.String(), ",") + `],"decryption":"none"}`,
	}).Error; err != nil {
		t.Fatalf("update inbound: %v", err)
	}
	assertBoundedSubscription(t, "s1")
}
