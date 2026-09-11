package sub

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func TestGetInboundsBySubIdOmitsExcludedInbounds(t *testing.T) {
	seedSubDB(t)

	visible := seedSubInbound(t, "sub-excl", "visible", 24401, 1, `{"network":"tcp","security":"none"}`)
	hidden := seedSubInbound(t, "sub-excl", "hidden", 24402, 2, `{"network":"tcp","security":"none"}`)
	if err := database.GetDB().Model(hidden).Update("exclude_from_sub", true).Error; err != nil {
		t.Fatalf("mark excluded: %v", err)
	}

	s := &SubService{}
	inbounds, err := s.getInboundsBySubId("sub-excl")
	if err != nil {
		t.Fatalf("getInboundsBySubId: %v", err)
	}
	if len(inbounds) != 1 || inbounds[0].Id != visible.Id {
		t.Fatalf("inbounds = %+v, want only visible id=%d", inbounds, visible.Id)
	}
	if inbounds[0].ExcludeFromSub {
		t.Fatal("visible inbound unexpectedly marked excludeFromSub")
	}
}
