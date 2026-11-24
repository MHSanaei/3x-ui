package service

import (
	"context"
	"os"
	"testing"
)

// The real test is the gate before every switch: it must pass on a site REALITY works with
// and fail when the target is unreachable. Needs an xray binary and internet:
//
//	XRAY_E2E_BINARY=/usr/local/x-ui/bin/xray-linux-amd64 go test ./internal/web/service -run TestRealityRealTestAgainstRealXray
func TestRealityRealTestAgainstRealXray(t *testing.T) {
	bin := os.Getenv("XRAY_E2E_BINARY")
	if bin == "" {
		t.Skip("XRAY_E2E_BINARY not set")
	}
	prev := realityRealTestBinary
	realityRealTestBinary = func() string { return bin }
	t.Cleanup(func() { realityRealTestBinary = prev })

	target := os.Getenv("REALITY_E2E_TARGET")
	if target == "" {
		target = "www.speedtest.net"
	}
	if err := xrayRealityRealTest(context.Background(), target+":443", target); err != nil {
		t.Fatalf("REALITY through %s: %v; want a reply", target, err)
	}
	if err := xrayRealityRealTest(context.Background(), "127.0.0.1:1", target); err == nil {
		t.Fatal("REALITY with an unreachable target passed; want an error")
	}
}
