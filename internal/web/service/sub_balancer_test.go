package service

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
)

var subBalancerLoggerOnce sync.Once

func setupSubBalancerDB(t *testing.T) {
	t.Helper()
	subBalancerLoggerOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Logf("CloseDB warning: %v", err)
		}
	})
}

func TestSubBalancerServiceCRUD(t *testing.T) {
	setupSubBalancerDB(t)
	svc := &SubBalancerService{}

	created, err := svc.Create(&model.SubBalancer{
		Remark: "auto", Strategy: "", InboundIds: []int{1, 2}, SortOrder: 0, Enabled: false,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Strategy != "random" {
		t.Fatalf("strategy = %q, want normalized random", created.Strategy)
	}
	if created.SortOrder != 1 {
		t.Fatalf("sortOrder = %d, want normalized 1", created.SortOrder)
	}
	stored, err := svc.Get(created.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Enabled {
		t.Fatal("explicit disabled balancer must be stored disabled")
	}

	second, err := svc.Create(&model.SubBalancer{
		Remark: "second", Strategy: "leastPing", InboundIds: []int{1}, SortOrder: 3, Enabled: true,
	})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	list, err := svc.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].Id != created.Id || list[1].Id != second.Id {
		t.Fatalf("list order = [%d %d], want [%d %d]", list[0].Id, list[1].Id, created.Id, second.Id)
	}

	enabledFalse := false
	updated, err := svc.Update(second.Id, &model.SubBalancer{
		Remark: "renamed", Strategy: "leastLoad", InboundIds: []int{2}, SortOrder: 2,
	}, &enabledFalse)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Remark != "renamed" || updated.Strategy != "leastLoad" || updated.SortOrder != 2 || updated.Enabled {
		t.Fatalf("update stored wrong row: %+v", updated)
	}
	after, err := svc.Get(second.Id)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if after.Enabled || after.Strategy != "leastLoad" || len(after.InboundIds) != 1 || after.InboundIds[0] != 2 {
		t.Fatalf("update did not persist: %+v", after)
	}

	if err := svc.Delete(created.Id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = svc.List()
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(list) != 1 || list[0].Id != second.Id {
		t.Fatalf("list after delete = %v", list)
	}
}

// roundRobin is a valid xray routing strategy (selects outbounds in order) and
// must pass the same validation as the other three.
func TestSubBalancerServiceRoundRobin(t *testing.T) {
	setupSubBalancerDB(t)
	svc := &SubBalancerService{}

	created, err := svc.Create(&model.SubBalancer{
		Remark: "rr", Strategy: "roundRobin", InboundIds: []int{1, 2}, SortOrder: 1, Enabled: true,
	})
	if err != nil {
		t.Fatalf("create roundRobin: %v", err)
	}
	if created.Strategy != "roundRobin" {
		t.Fatalf("strategy = %q, want roundRobin", created.Strategy)
	}
	stored, err := svc.Get(created.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Strategy != "roundRobin" {
		t.Fatalf("stored strategy = %q, want roundRobin", stored.Strategy)
	}
}

// Deleting a missing balancer reports not-found instead of success:true, so
// a stale UI row can't claim a delete that touched nothing.
func TestSubBalancerServiceDeleteNotFound(t *testing.T) {
	setupSubBalancerDB(t)
	svc := &SubBalancerService{}
	if err := svc.Delete(999); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Delete(999) = %v, want a not-found error", err)
	}
}

func TestSubBalancerServiceValidation(t *testing.T) {
	setupSubBalancerDB(t)
	svc := &SubBalancerService{}

	cases := []struct {
		name string
		row  model.SubBalancer
		want string
	}{
		{"empty remark", model.SubBalancer{Strategy: "random", InboundIds: []int{1}}, "remark is required"},
		{"bad strategy", model.SubBalancer{Remark: "x", Strategy: "fastest", InboundIds: []int{1}}, "invalid balancer strategy"},
		{"no inbounds", model.SubBalancer{Remark: "x", Strategy: "random"}, "at least one inbound"},
		{"long remark", model.SubBalancer{Remark: strings.Repeat("x", 257), Strategy: "random", InboundIds: []int{1}}, "max 256"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(&tc.row)
			if err == nil {
				t.Fatal("create must fail")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

// Weights are a leastLoad-only knob; xray silently ignores costs elsewhere, so
// storing them would pretend a knob exists. Non-positive weights are rejected
// rather than defaulted — a zero usually means a typo'd "never pick this node".
func TestSubBalancerServiceWeightValidation(t *testing.T) {
	setupSubBalancerDB(t)
	svc := &SubBalancerService{}

	if _, err := svc.Create(&model.SubBalancer{
		Remark: "w", Strategy: "random", InboundIds: []int{1},
		MemberWeights: map[int]float64{1: 0.5},
	}); err == nil || !strings.Contains(err.Error(), "leastLoad strategy") {
		t.Fatalf("weights with random = %v, want leastLoad-strategy error", err)
	}

	if _, err := svc.Create(&model.SubBalancer{
		Remark: "w", Strategy: "leastLoad", InboundIds: []int{1},
		MemberWeights: map[int]float64{1: -0.5},
	}); err == nil || !strings.Contains(err.Error(), "greater than 0") {
		t.Fatalf("negative weight = %v, must be rejected", err)
	}

	stray, err := svc.Create(&model.SubBalancer{
		Remark: "stray", Strategy: "leastLoad", InboundIds: []int{1, 2},
		MemberWeights: map[int]float64{2: 0.25, 99: 3.0},
	})
	if err != nil {
		t.Fatalf("create with stray weight id: %v", err)
	}
	stored, err := svc.Get(stray.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(stored.MemberWeights) != 1 || stored.MemberWeights[2] != 0.25 {
		t.Fatalf("memberWeights = %v, want only {2:0.25} (id 99 dropped)", stored.MemberWeights)
	}

	reweighted, err := svc.Update(stray.Id, &model.SubBalancer{
		Remark: "stray", Strategy: "leastLoad", InboundIds: []int{1, 2},
		MemberWeights: map[int]float64{1: 2.5}, SortOrder: 1,
	}, nil)
	if err != nil {
		t.Fatalf("update weights: %v", err)
	}
	if reweighted.MemberWeights[1] != 2.5 || len(reweighted.MemberWeights) != 1 {
		t.Fatalf("updated memberWeights = %v, want {1:2.5}", reweighted.MemberWeights)
	}

	cleared, err := svc.Update(stray.Id, &model.SubBalancer{
		Remark: "stray", Strategy: "leastLoad", InboundIds: []int{1, 2}, SortOrder: 1,
	}, nil)
	if err != nil {
		t.Fatalf("update without weights: %v", err)
	}
	if cleared.MemberWeights != nil {
		t.Fatalf("absent memberWeights must clear stored weights, got %v", cleared.MemberWeights)
	}
}
