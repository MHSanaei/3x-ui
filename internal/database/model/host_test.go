package model

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

// TestHostTableName locks the table name the rest of the feature (queries,
// prune, migration) keys off.
func TestHostTableName(t *testing.T) {
	if got := (Host{}).TableName(); got != "hosts" {
		t.Fatalf("Host.TableName() = %q, want hosts", got)
	}
}

// TestHostValidation locks the struct-tag constraints enforced by the request
// binder (middleware.BindAndValidate -> validate.Struct).
func TestHostValidation(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())

	valid := Host{InboundId: 1, Remark: "cdn-front", Port: 8443, Security: "tls", MihomoIpVersion: "dual"}
	if err := v.Struct(valid); err != nil {
		t.Fatalf("valid host rejected: %v", err)
	}

	bad := []struct {
		name string
		h    Host
	}{
		{"missing inbound", Host{Remark: "ok"}},
		{"empty remark", Host{InboundId: 1, Remark: ""}},
		{"remark too long", Host{InboundId: 1, Remark: strings.Repeat("x", 257)}},
		{"port too high", Host{InboundId: 1, Remark: "ok", Port: 70000}},
		{"port negative", Host{InboundId: 1, Remark: "ok", Port: -1}},
		{"bad security", Host{InboundId: 1, Remark: "ok", Security: "bogus"}},
		{"bad mihomo ip version", Host{InboundId: 1, Remark: "ok", MihomoIpVersion: "nope"}},
	}
	for _, tc := range bad {
		if err := v.Struct(tc.h); err == nil {
			t.Fatalf("%s: expected validation error, got nil", tc.name)
		}
	}
}
