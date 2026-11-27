package service

import (
	"fmt"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestOperationalNodeListPreservesConfiguration(t *testing.T) {
	testOperationalNodeList(t, initTrafficTestDB(t))
}

func TestOperationalNodeListPreservesConfigurationPostgres(t *testing.T) {
	testOperationalNodeList(t, durablePostgresDB(t))
}

func testOperationalNodeList(t *testing.T, db *gorm.DB) {
	t.Helper()
	enableNodeTokenEncryption(t)
	service := &NodeService{}
	first := &model.Node{Name: "op-first", Address: "127.0.0.1", Port: 2096, ApiToken: "fixture-a", Enable: true}
	second := &model.Node{Name: "op-second", Address: "127.0.0.1", Port: 2097, ApiToken: "fixture-b", Enable: true}
	for _, node := range []*model.Node{first, second} {
		if err := service.Create(node); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(second).Updates(map[string]any{"enable": false, "api_token": "enc:bad-token"}).Error; err != nil {
		t.Fatal(err)
	}
	nodes, err := service.ListOperationalNodes()
	if err != nil {
		t.Fatal(err)
	}
	var got []*model.Node
	for _, n := range nodes {
		if n.Id == first.Id || n.Id == second.Id {
			got = append(got, n)
		}
	}
	if len(got) != 2 || got[0].Id != first.Id || got[1].Id != second.Id {
		t.Fatalf("nodes missing or out of ID order: %v", got)
	}
	if got[0].ApiToken != first.ApiToken || got[0].Port != first.Port || !got[0].Enable ||
		got[1].ApiToken != "" || got[1].LastError == "" || got[1].Enable {
		t.Fatal("operational node configuration or token error was lost")
	}
}

func BenchmarkOperationalNodes(b *testing.B) {
	for _, count := range []int{1000, 10000, 50000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			setupScaleDB(b)
			ds := seedScaleDataset(b, count, 4)
			db := database.GetDB()
			node := &model.Node{Name: "bench-operational", Address: "127.0.0.1", Port: 2096}
			if err := db.Create(node).Error; err != nil {
				b.Fatal(err)
			}
			if err := db.Model(&model.Inbound{}).Where("id IN ?", ds.inboundIds).Update("node_id", node.Id).Error; err != nil {
				b.Fatal(err)
			}
			svc := &NodeService{}
			for _, tc := range []struct {
				name string
				list func() ([]*model.Node, error)
			}{{"dashboard", svc.GetAll}, {"operational", svc.ListOperationalNodes}} {
				b.Run(tc.name, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						if _, err := tc.list(); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
