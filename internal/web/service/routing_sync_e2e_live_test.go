//go:build e2e

package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func liveDBPath(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("XUI_E2E_ROOT"); root != "" {
		return filepath.Join(root, "x-ui", "x-ui.db")
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "x-ui", "x-ui.db")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	t.Fatal("could not find x-ui/x-ui.db; set XUI_E2E_ROOT")
	return ""
}

func initLiveDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(liveDBPath(t)); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func backupTemplate(t *testing.T) string {
	t.Helper()
	got, err := (&XraySettingService{}).GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("GetXrayConfigTemplate: %v", err)
	}
	return got
}

func restoreTemplate(t *testing.T, raw string) {
	t.Helper()
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", raw); err != nil {
		t.Fatalf("restore template: %v", err)
	}
}

func seedRoutingTemplate(t *testing.T, tag string) {
	t.Helper()
	template := fmt.Sprintf(`{
		"routing": {
			"rules": [
				{"type":"field","inboundTag":["api"],"outboundTag":"api"},
				{"type":"field","inboundTag":["%s"],"outboundTag":"direct"},
				{"type":"field","inboundTag":["%s"],"domain":["example.com"],"outboundTag":"blocked"},
				{"type":"field","inboundTag":["%s","in-99999-tcp"],"outboundTag":"proxy"}
			]
		},
		"outbounds": [
			{"tag":"direct","protocol":"freedom"},
			{"tag":"blocked","protocol":"blackhole"},
			{"tag":"proxy","protocol":"freedom"}
		]
	}`, tag, tag, tag)
	seedXrayTemplate(t, template)
}

func createTestInbound(t *testing.T) *model.Inbound {
	t.Helper()
	inboundSvc := &InboundService{}
	ib := &model.Inbound{
		Remark:         "e2e-routing-sync",
		Enable:         true,
		Port:           59201,
		Protocol:       model.Protocol("vless"),
		Tag:            "e2e-routing-tag",
		Listen:         "",
		Settings:       `{"clients":[],"decryption":"none","fallbacks":[]}`,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
		Sniffing:       `{"enabled":false,"destOverride":["http","tls","quic","fakedns"]}`,
	}
	created, _, err := inboundSvc.AddInbound(ib)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	t.Cleanup(func() {
		_, _ = inboundSvc.DelInbound(created.Id)
	})
	return created
}

func reloadInbound(t *testing.T, id int) *model.Inbound {
	t.Helper()
	ib, err := (&InboundService{}).GetInbound(id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	return ib
}

func TestLiveRoutingSync_RenameAndDelete(t *testing.T) {
	initLiveDB(t)
	ib := createTestInbound(t)
	oldTag := ib.Tag
	newTag := "e2e-routing-renamed"

	backup := backupTemplate(t)
	t.Cleanup(func() { restoreTemplate(t, backup) })

	seedRoutingTemplate(t, oldTag)

	inboundSvc := &InboundService{}
	xraySvc := &XraySettingService{}

	ib = reloadInbound(t, ib.Id)
	ib.Tag = newTag
	if _, _, err := inboundSvc.UpdateInbound(ib); err != nil {
		t.Fatalf("UpdateInbound rename: %v", err)
	}

	got, err := xraySvc.GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("GetXrayConfigTemplate: %v", err)
	}
	rules := routingRulesFromTemplate(t, got)
	if len(rules) != 4 {
		t.Fatalf("after rename: rules len = %d, want 4", len(rules))
	}
	if tags := readInboundTags(rules[1]["inboundTag"]); tags[0] != newTag {
		t.Fatalf("inbound-only rule tag = %v, want %q", tags, newTag)
	}
	multi := rules[2]
	if tags := readInboundTags(multi["inboundTag"]); len(tags) != 1 || tags[0] != newTag {
		t.Fatalf("multi-condition inboundTag = %v, want [%q]", tags, newTag)
	}
	if _, ok := multi["domain"]; !ok {
		t.Fatal("multi-condition rule lost domain matcher")
	}
	if tags := readInboundTags(rules[3]["inboundTag"]); len(tags) != 2 || tags[0] != newTag {
		t.Fatalf("dual inbound rule = %v", tags)
	}

	row := reloadInbound(t, ib.Id)
	if _, err := inboundSvc.DelInbound(row.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	// Prevent cleanup from trying to delete again.
	t.Cleanup(func() {})

	got, err = xraySvc.GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("GetXrayConfigTemplate after delete: %v", err)
	}
	rules = routingRulesFromTemplate(t, got)
	if len(rules) != 3 {
		t.Fatalf("after delete: rules len = %d, want 3 (api + domain-only + dual remainder)", len(rules))
	}
	if tags := readInboundTags(rules[0]["inboundTag"]); tags[0] != "api" {
		t.Fatalf("api rule = %v", tags)
	}
	domainRule := rules[1]
	if _, ok := domainRule["inboundTag"]; ok {
		t.Fatalf("domain rule should have inboundTag removed, got %#v", domainRule["inboundTag"])
	}
	if domainRule["outboundTag"] != "blocked" {
		t.Fatalf("domain rule outbound = %v", domainRule["outboundTag"])
	}
	proxyRule := rules[2]
	if tags := readInboundTags(proxyRule["inboundTag"]); len(tags) != 1 || tags[0] != "in-99999-tcp" {
		t.Fatalf("proxy rule after delete = %v, want [in-99999-tcp]", tags)
	}

	t.Logf("live e2e OK: renamed %q -> %q; delete removed inbound-only rule and stripped tags from multi-condition rules", oldTag, newTag)
}

// TestLiveRoutingSync_CustomToAutoTag clears a custom tag (UI auto mode) and
// expects routing rules to follow the newly generated auto tag.
func TestLiveRoutingSync_CustomToAutoTag(t *testing.T) {
	initLiveDB(t)
	customTag := "e2e-custom-tag"
	port := 59203

	backup := backupTemplate(t)
	t.Cleanup(func() { restoreTemplate(t, backup) })

	db := database.GetDB()
	db.Where("remark = ?", "e2e-custom-to-auto").Delete(&model.Inbound{})

	inboundSvc := &InboundService{}
	ib := &model.Inbound{
		Remark:         "e2e-custom-to-auto",
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
	t.Cleanup(func() { _, _ = inboundSvc.DelInbound(created.Id) })

	seedRoutingTemplate(t, customTag)
	xraySvc := &XraySettingService{}

	row := reloadInbound(t, created.Id)
	row.Tag = "" // frontend auto mode submits empty tag
	if _, _, err := inboundSvc.UpdateInbound(row); err != nil {
		t.Fatalf("UpdateInbound clear custom tag: %v", err)
	}
	row = reloadInbound(t, created.Id)
	autoTag := row.Tag
	if autoTag == "" || autoTag == customTag {
		t.Fatalf("expected auto-generated tag, got %q", autoTag)
	}
	t.Logf("custom tag %q -> auto tag %q", customTag, autoTag)

	got, err := xraySvc.GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("GetXrayConfigTemplate: %v", err)
	}
	rules := routingRulesFromTemplate(t, got)
	if len(rules) != 4 {
		t.Fatalf("after custom->auto: rules len = %d, want 4", len(rules))
	}
	if tags := readInboundTags(rules[1]["inboundTag"]); tags[0] != autoTag {
		t.Fatalf("inbound-only rule tag = %v, want %q", tags, autoTag)
	}
	multi := rules[2]
	if tags := readInboundTags(multi["inboundTag"]); len(tags) != 1 || tags[0] != autoTag {
		t.Fatalf("multi-condition inboundTag = %v, want [%q]", tags, autoTag)
	}
	if _, ok := multi["domain"]; !ok {
		t.Fatal("multi-condition rule lost domain matcher")
	}
	if tags := readInboundTags(rules[3]["inboundTag"]); len(tags) != 2 || tags[0] != autoTag {
		t.Fatalf("dual inbound rule = %v", tags)
	}
	for _, rule := range rules {
		for _, tag := range readInboundTags(rule["inboundTag"]) {
			if tag == customTag {
				t.Fatalf("stale custom tag %q still in routing rules", customTag)
			}
		}
	}

	// Delete after auto tag: multi-condition cleanup should still work.
	if _, err := inboundSvc.DelInbound(row.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	t.Cleanup(func() {})

	got, err = xraySvc.GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("GetXrayConfigTemplate after delete: %v", err)
	}
	rules = routingRulesFromTemplate(t, got)
	if len(rules) != 3 {
		t.Fatalf("after delete: rules len = %d, want 3", len(rules))
	}
	domainRule := rules[1]
	if _, ok := domainRule["inboundTag"]; ok {
		t.Fatalf("domain rule should have inboundTag removed, got %#v", domainRule["inboundTag"])
	}
	if tags := readInboundTags(rules[2]["inboundTag"]); len(tags) != 1 || tags[0] != "in-99999-tcp" {
		t.Fatalf("proxy rule after delete = %v", tags)
	}

	t.Logf("live e2e OK: custom->auto propagated to all rule shapes; delete cleaned multi-condition rules")
}

func TestLiveRoutingSync_TemplateJSONValid(t *testing.T) {
	initLiveDB(t)
	got, err := (&XraySettingService{}).GetXrayConfigTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(got), &cfg); err != nil {
		t.Fatalf("template not valid JSON: %v", err)
	}
}
