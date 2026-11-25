package database

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const legacyFinalRuleDelayTemplate = `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"finalRules":[` +
	`{"action":"allow","ip":["10.0.0.0/8"],"blockDelay":"1000-2000"},` +
	`{"action":"Block","blockDelay":"5000-10000","delay":"1-2"}]}}]}`

func finalRulesOfFirstOutbound(t *testing.T, raw string) (json.RawMessage, []any) {
	t.Helper()
	var cfg struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("template is not JSON: %v", err)
	}
	var outbound struct {
		Settings struct {
			FinalRules []any `json:"finalRules"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(cfg.Outbounds[0], &outbound); err != nil {
		t.Fatalf("outbound is not JSON: %v", err)
	}
	return cfg.Outbounds[0], outbound.Settings.FinalRules
}

func TestRewriteFreedomFinalRuleDelay(t *testing.T) {
	before, _ := finalRulesOfFirstOutbound(t, legacyFinalRuleDelayTemplate)
	err := xray.ValidateOutboundConfig(before)
	if err == nil || !strings.Contains(err.Error(), "only supported for block finalRules") {
		t.Fatalf("expected the vendored core to refuse a delay on an allow rule, got %v", err)
	}

	updated, changed, err := rewriteFreedomFinalRuleDelay(legacyFinalRuleDelayTemplate)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !changed {
		t.Fatal("the allow rule kept the blockDelay the core refuses")
	}
	after, rules := finalRulesOfFirstOutbound(t, updated)
	if err := xray.ValidateOutboundConfig(after); err != nil {
		t.Fatalf("rewritten outbound still refused by xray-core: %v", err)
	}
	want := []any{
		map[string]any{"action": "allow", "ip": []any{"10.0.0.0/8"}},
		map[string]any{"action": "Block", "delay": "5000-10000"},
	}
	if !reflect.DeepEqual(rules, want) {
		t.Fatalf("finalRules = %v, want %v", rules, want)
	}

	if _, again, err := rewriteFreedomFinalRuleDelay(updated); err != nil || again {
		t.Fatalf("second pass changed=%v err=%v, want a no-op", again, err)
	}
}

func TestFreedomFinalRuleDelayFixRunsOnUpgradedPanels(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := InitDB(config.GetDBPath()); err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skipf("sqlite needs cgo: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	seedTemplate(t, legacyFinalRuleDelayTemplate)
	if err := db.Where("seeder_name = ?", "FreedomFinalRuleDelayFix").
		Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}

	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}

	outbound, _ := finalRulesOfFirstOutbound(t, storedTemplate(t))
	if err := xray.ValidateOutboundConfig(outbound); err != nil {
		t.Fatalf("stored template still refused by xray-core: %v", err)
	}
	var count int64
	if err := db.Model(&model.HistoryOfSeeders{}).
		Where("seeder_name = ?", "FreedomFinalRuleDelayFix").Count(&count).Error; err != nil {
		t.Fatalf("count seeder rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("FreedomFinalRuleDelayFix recorded %d times, want 1", count)
	}
}
