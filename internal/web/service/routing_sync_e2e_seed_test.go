//go:build e2e

package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestLiveRoutingSync_SeedBrowserDemo prepares routing rules for manual/browser verification.
// Uses existing inbound "browser-tag-test" (tag test-inbound-tag) when present; otherwise creates demo inbound.
// Run: go test ./internal/web/service -tags=e2e -run TestLiveRoutingSync_SeedBrowserDemo -v -count=1
func TestLiveRoutingSync_SeedBrowserDemo(t *testing.T) {
	initLiveDB(t)
	tag := "test-inbound-tag"
	remark := "browser-tag-test"

	db := database.GetDB()
	var existing model.Inbound
	if err := db.Where("remark = ?", remark).First(&existing).Error; err != nil {
		tag = "demo-routing-tag"
		remark = "demo-routing-inbound"
		db.Where("remark = ?", remark).Delete(&model.Inbound{})

		inboundSvc := &InboundService{}
		ib := &model.Inbound{
			Remark:         remark,
			Enable:         true,
			Port:           59202,
			Protocol:       model.Protocol("vless"),
			Tag:            tag,
			Settings:       `{"clients":[],"decryption":"none","fallbacks":[]}`,
			StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
			Sniffing:       `{"enabled":false,"destOverride":["http","tls","quic","fakedns"]}`,
		}
		created, _, err := inboundSvc.AddInbound(ib)
		if err != nil {
			t.Fatalf("AddInbound: %v", err)
		}
		t.Logf("created demo inbound id=%d remark=%q tag=%q", created.Id, remark, tag)
	} else {
		t.Logf("using existing inbound id=%d remark=%q tag=%q", existing.Id, remark, tag)
	}

	template := fmt.Sprintf(`{
		"routing": {
			"domainStrategy": "AsIs",
			"rules": [
				{"type":"field","inboundTag":["api"],"outboundTag":"api"},
				{"type":"field","inboundTag":["%s"],"outboundTag":"direct"},
				{"type":"field","inboundTag":["%s"],"domain":["example.com"],"outboundTag":"blocked"},
				{"type":"field","inboundTag":["%s","in-99999-tcp"],"outboundTag":"proxy"}
			]
		},
		"outbounds": [
			{"tag":"direct","protocol":"freedom","settings":{}},
			{"tag":"blocked","protocol":"blackhole","settings":{}},
			{"tag":"proxy","protocol":"freedom","settings":{}}
		]
	}`, tag, tag, tag)
	seedXrayTemplate(t, template)

	t.Logf("seeded routing rules for tag=%q (api + 3 demo rules)", tag)
}

// TestLiveRoutingSync_SeedCustomToAutoDemo prepares an inbound with a custom tag
// and routing rules for browser verification of clearing tag -> auto-generate.
// Run: go test ./internal/web/service -tags=e2e -run TestLiveRoutingSync_SeedCustomToAutoDemo -v -count=1
func TestLiveRoutingSync_SeedCustomToAutoDemo(t *testing.T) {
	initLiveDB(t)
	customTag := "my-custom-routing-tag"
	remark := "auto-tag-test"
	port := 59203

	db := database.GetDB()
	found := false
	for _, candidate := range []string{"browser-auto-test", "auto-tag-test"} {
		var existing model.Inbound
		if err := db.Where("remark = ?", candidate).First(&existing).Error; err == nil {
			remark = candidate
			customTag = existing.Tag
			port = existing.Port
			found = true
			t.Logf("using existing inbound id=%d remark=%q tag=%q port=%d", existing.Id, remark, customTag, port)
			break
		}
	}
	if !found {
		db.Where("remark = ?", remark).Delete(&model.Inbound{})

		inboundSvc := &InboundService{}
		ib := &model.Inbound{
			Remark:         remark,
			Enable:         true,
			Port:           port,
			Protocol:       model.Protocol("vless"),
			Tag:            customTag,
			Settings:       `{"clients":[],"decryption":"none","fallbacks":[]}`,
			StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
			Sniffing:       `{"enabled":false,"destOverride":["http","tls","quic","fakedns"]}`,
		}
		created, _, err := inboundSvc.AddInbound(ib)
		if err != nil {
			t.Fatalf("AddInbound: %v", err)
		}
		t.Logf("created inbound id=%d remark=%q customTag=%q port=%d", created.Id, remark, customTag, port)
	}

	template := fmt.Sprintf(`{
		"routing": {
			"domainStrategy": "AsIs",
			"rules": [
				{"type":"field","inboundTag":["api"],"outboundTag":"api"},
				{"type":"field","inboundTag":["%s"],"outboundTag":"direct"},
				{"type":"field","inboundTag":["%s"],"domain":["example.com"],"outboundTag":"blocked"},
				{"type":"field","inboundTag":["%s","in-99999-tcp"],"outboundTag":"proxy"}
			]
		},
		"outbounds": [
			{"tag":"direct","protocol":"freedom","settings":{}},
			{"tag":"blocked","protocol":"blackhole","settings":{}},
			{"tag":"proxy","protocol":"freedom","settings":{}}
		]
	}`, customTag, customTag, customTag)
	seedXrayTemplate(t, template)

	expectedAuto := fmt.Sprintf("in-%d-tcp", port)
	t.Logf("seeded routing rules for customTag=%q remark=%q (expected auto tag %q after clear)", customTag, remark, expectedAuto)
}
