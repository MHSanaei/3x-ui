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

	updated, err := svc.Update(second.Id, &model.SubBalancer{
		Remark: "renamed", Strategy: "leastLoad", InboundIds: []int{2}, SortOrder: 2, Enabled: false,
	})
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
