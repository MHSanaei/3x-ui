package middleware

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The inbound add/update handlers validate the body with this validator, so a protocol
// missing from Inbound.Protocol's oneof makes every save of that protocol fail.
func TestValidateAcceptsMasqueInboundProtocol(t *testing.T) {
	if err := validate.StructPartial(model.Inbound{Protocol: model.MASQUE}, "Protocol"); err != nil {
		t.Fatalf("validate rejected a masque inbound: %v", err)
	}
	if err := validate.StructPartial(model.Inbound{Protocol: "dokodemo"}, "Protocol"); err == nil {
		t.Fatal("validate accepted an unknown protocol; the oneof no longer guards it")
	}
}
