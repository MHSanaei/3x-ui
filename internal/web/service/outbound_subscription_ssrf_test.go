package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

func TestSubscriptionFetchClientBlocksPrivateDial(t *testing.T) {
	setupSettingTestDB(t)
	client := (&OutboundSubscriptionService{}).subscriptionFetchClient(5 * time.Second)

	ctx := netsafe.ContextWithAllowPrivate(context.Background(), false)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1/", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	_, err = client.Do(req)
	if err == nil {
		t.Fatal("the fetch client dialed a private address instead of blocking it")
	}
	if !strings.Contains(err.Error(), "blocked private") {
		t.Fatalf("expected an SSRF-guard block, got a plain dial error: %v", err)
	}
}
