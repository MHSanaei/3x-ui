package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func initInboundFeaturesTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
	prevMgr := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{
		APIPort:        func() int { return 0 },
		SetNeedRestart: func() {},
	}))
	t.Cleanup(func() { runtime.SetManager(prevMgr) })
}

// orderFeatureInbound builds a minimal valid vless inbound the UpdateInbound
// path accepts: no TLS, no clients, plain TCP. The tag mirrors the auto-tag
// the service would generate (AddInbound resolves it; a raw row insert
// would leave the unique tag column blank and collide).
func orderFeatureInbound(t *testing.T, user int, port int, remark string, subSortIndex int) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		UserId:         user,
		Enable:         false,
		Remark:         remark,
		Port:           port,
		Protocol:       model.VLESS,
		Tag:            fmt.Sprintf("in-%d-tcp", port),
		Settings:       `{"clients":[],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
		Sniffing:       `{}`,
		SubSortIndex:   subSortIndex,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	return ib
}

// TestGetInboundsSlimRespectsInboundListOrder is the behaviour the panel list
// depends on: the configured default order (a global setting, so it applies to
// local and node-adopted inbounds alike) must reach the slim endpoint.
func TestGetInboundsSlimRespectsInboundListOrder(t *testing.T) {
	initInboundFeaturesTestDB(t)
	svc := &InboundService{}
	setting := &SettingService{}

	a := orderFeatureInbound(t, 1, 443, "zeta", 3)
	b := orderFeatureInbound(t, 1, 8443, "alpha", 1)

	ids := func() []int {
		list, err := svc.GetInboundsSlim(1)
		if err != nil {
			t.Fatalf("GetInboundsSlim: %v", err)
		}
		out := make([]int, 0, len(list))
		for _, ib := range list {
			out = append(out, ib.Id)
		}
		return out
	}

	// The historical default stays id order.
	if got, want := ids(), []int{a.Id, b.Id}; !slices.Equal(got, want) {
		t.Fatalf("default order = %v, want %v", got, want)
	}

	if err := setting.setString("inboundListOrder", "port"); err != nil {
		t.Fatalf("set order: %v", err)
	}
	if got, want := ids(), []int{a.Id, b.Id}; !slices.Equal(got, want) {
		t.Fatalf("port order = %v, want %v (443 before 8443)", got, want)
	}

	if err := setting.setString("inboundListOrder", "remark"); err != nil {
		t.Fatalf("set order: %v", err)
	}
	if got, want := ids(), []int{b.Id, a.Id}; !slices.Equal(got, want) {
		t.Fatalf("remark order = %v, want %v (alpha before zeta)", got, want)
	}

	if err := setting.setString("inboundListOrder", "subSortIndex"); err != nil {
		t.Fatalf("set order: %v", err)
	}
	if got, want := ids(), []int{b.Id, a.Id}; !slices.Equal(got, want) {
		t.Fatalf("subSortIndex order = %v, want %v (index 1 before 3)", got, want)
	}
}

// TestUpdateAllSettingInboundListOrder catches an invalid or missing order
// mode at save time so a bad value can never corrupt the list query.
func TestUpdateAllSettingInboundListOrder(t *testing.T) {
	initInboundFeaturesTestDB(t)
	svc := &SettingService{}
	all, err := svc.GetAllSetting()
	if err != nil {
		t.Fatalf("GetAllSetting: %v", err)
	}
	all.InboundListOrder = "buddy-sort"
	if err := svc.UpdateAllSetting(all, SecretClears{}); err == nil ||
		err.Error() != "inbound list order must be id, remark, port, protocol, or subSortIndex" {
		t.Fatalf("invalid order: err = %v, want the exact validation error", err)
	}
	all.InboundListOrder = ""
	if err := svc.UpdateAllSetting(all, SecretClears{}); err != nil {
		t.Fatalf("blank order must normalize, got %v", err)
	}
	if got, _ := svc.GetInboundListOrder(); got != "id" {
		t.Fatalf("GetInboundListOrder = %q, want id", got)
	}
}

// TestUpdateAllSettingNodeRoutingRules rejects a node routing policy that is
// not a JSON array of non-empty rule objects.
func TestUpdateAllSettingNodeRoutingRules(t *testing.T) {
	initInboundFeaturesTestDB(t)
	svc := &SettingService{}
	all, err := svc.GetAllSetting()
	if err != nil {
		t.Fatalf("GetAllSetting: %v", err)
	}
	all.NodeRoutingRules = `{"not":"an array"}`
	if err := svc.UpdateAllSetting(all, SecretClears{}); err == nil ||
		err.Error() != "node routing rules must be a JSON array of routing rules" {
		t.Fatalf("non-array policy: err = %v, want the array validation error", err)
	}
	all.NodeRoutingRules = `[{}]`
	if err := svc.UpdateAllSetting(all, SecretClears{}); err == nil ||
		err.Error() != "node routing rule 0 is empty" {
		t.Fatalf("empty rule object: err = %v, want the empty-rule error", err)
	}
	all.NodeRoutingRules = `[{"type":"field","outboundTag":"direct"}]`
	if err := svc.UpdateAllSetting(all, SecretClears{}); err != nil {
		t.Fatalf("valid policy must save, got %v", err)
	}
}

// seedRoutingTemplate stores a minimal valid xray template carrying one
// routing rule referencing tag, so the rename propagation has a real rule to
// rewrite. It avoids geodata (geoip:/geosite: tokens) so CheckXrayConfig
// passes without the .dat assets a real install ships.
func seedRoutingTemplate(t *testing.T, tag string) {
	t.Helper()
	setting := &SettingService{}
	template := `{
		"log": {"loglevel": "warning"},
		"inbounds": [],
		"outbounds": [{"tag": "direct", "protocol": "freedom"}],
		"routing": {
			"domainStrategy": "as-is",
			"final": "direct",
			"rules": [
				{"type": "field", "inboundTag": ["` + tag + `"], "outboundTag": "direct"}
			]
		}
	}`
	if err := setting.saveSetting("xrayTemplateConfig", template); err != nil {
		t.Fatalf("save template: %v", err)
	}
}

// TestCheckTagChange drives the pre-flight checks through a real DB and
// template: in-use tags, unchanged targets and routing references.
func TestCheckTagChange(t *testing.T) {
	initInboundFeaturesTestDB(t)
	svc := &InboundService{}

	ib := orderFeatureInbound(t, 1, 100, "probe", 0)
	seedRoutingTemplate(t, ib.Tag)

	// Same tag: no-op, valid.
	check, err := svc.CheckTagChange(1, ib.Id, ib.Tag)
	if err != nil || !check.Unchanged || !check.Valid {
		t.Fatalf("unchanged target: check=%+v err=%v, want unchanged+valid", check, err)
	}

	// Blank target.
	check, err = svc.CheckTagChange(1, ib.Id, "   ")
	if err != nil || check.Valid || check.Reason != "blank" {
		t.Fatalf("blank target: check=%+v err=%v, want reason=blank", check, err)
	}

	// A tag another inbound owns.
	other := orderFeatureInbound(t, 1, 101, "other", 0)
	check, err = svc.CheckTagChange(1, ib.Id, other.Tag)
	if err != nil || check.Valid || check.Reason != "in-use" {
		t.Fatalf("in-use target: check=%+v err=%v, want reason=in-use", check, err)
	}

	// Free tag: valid, and the reference count covers the one rule that names
	// the inbound's current tag.
	check, err = svc.CheckTagChange(1, ib.Id, "renamed-100")
	if err != nil {
		t.Fatalf("free target: %v", err)
	}
	if !check.Valid || check.RoutingRefs != 1 {
		t.Fatalf("free target: check=%+v, want valid with 1 routing ref", check)
	}
}

// TestChangeInboundTagRewritesRouting is the end-to-end safe rename: the tag
// moves, routing rules that referenced the old tag are rewritten, and the
// rename is reported through the proven UpdateInbound path.
func TestChangeInboundTagRewritesRouting(t *testing.T) {
	initInboundFeaturesTestDB(t)
	ib := orderFeatureInbound(t, 1, 200, "renamer", 0)
	oldTag := ib.Tag
	seedRoutingTemplate(t, oldTag)

	needRestart, err := (&InboundService{}).ChangeInboundTag(1, ib.Id, "renamed-200")
	if err != nil {
		t.Fatalf("ChangeInboundTag: %v", err)
	}
	if !needRestart {
		t.Fatal("a routing rewrite must flag a restart")
	}

	var stored model.Inbound
	if err := database.GetDB().First(&stored, ib.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if stored.Tag != "renamed-200" {
		t.Fatalf("inbound tag = %q, want renamed-200", stored.Tag)
	}
	template, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	if !strings.Contains(template, `"renamed-200"`) || strings.Contains(template, oldTag) {
		t.Fatalf("routing not rewritten: template=%s", template)
	}
}

// TestMergeMasterRoutingRules checks the node-side merge: the policy leads the
// node's own rules, the pinned api stats rule stays first, and panel-only
// keys are stripped; blank or broken policy must leave the config untouched.
func TestMergeMasterRoutingRules(t *testing.T) {
	base := `{"rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"},{"type":"field","domain":["x.test"],"outboundTag":"direct"}]}`

	// No policy: byte-identical config.
	if got := mergeMasterRoutingRules(json_util.RawMessage(base), ""); string(got) != base {
		t.Fatalf("blank policy changed the config: %s", got)
	}
	// Unparseable policy: config kept as-is rather than clobbered.
	if got := mergeMasterRoutingRules(json_util.RawMessage(base), "{broken"); string(got) != base {
		t.Fatalf("broken policy changed the config: %s", got)
	}

	policy := `[{"type":"field","network":"udp","outboundTag":"direct","comment":"panel key","enabled":false},{"type":"field","domain":["y.test"],"outboundTag":"direct"}]`
	got := mergeMasterRoutingRules(json_util.RawMessage(base), policy)
	var merged struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(got, &merged); err != nil {
		t.Fatalf("merge produced unparseable routing: %v", err)
	}
	// The disabled rule is dropped; only the enabled one is merged in.
	if len(merged.Rules) != 3 {
		t.Fatalf("merged rule count = %d, want 3 (api + 1 enabled policy + original)", len(merged.Rules))
	}
	// Ensure the disabled rule (network:udp) did not make it through.
	for _, r := range merged.Rules {
		if n, _ := r["network"].(string); n == "udp" {
			t.Fatalf("disabled policy rule leaked into merged config: %v", r)
		}
	}
	if merged.Rules[0]["outboundTag"] != "api" {
		t.Fatalf("api stats rule no longer pinned first: %v", merged.Rules[0])
	}
	first := merged.Rules[1]
	if first["outboundTag"] != "direct" {
		t.Fatalf("policy rule not second: %v", first)
	}
	if _, has := first["comment"]; has {
		t.Fatalf("panel-only key survived the merge: %v", first)
	}
	if _, has := first["enabled"]; has {
		t.Fatalf("panel-only enabled key survived the merge: %v", first)
	}
}
