package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// UpdateInbound copies the payload onto the stored row field by field, so a
// column it does not copy is silently dropped: the API answers with the value
// it was sent while the row keeps the old one, and the subscription then ships
// a direct connection around the chain the admin assigned.
func TestUpdateInbound_PersistsChainAssignment(t *testing.T) {
	cases := []struct {
		name       string
		proxyId    int
		selectorId int
		wantProxy  int
		wantSelect int
	}{
		{name: "single hop", proxyId: 1, selectorId: 0, wantProxy: 1, wantSelect: 0},
		{name: "selector", proxyId: 0, selectorId: 1, wantProxy: 0, wantSelect: 1},
		{name: "cleared", proxyId: 0, selectorId: 0, wantProxy: 0, wantSelect: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupChainDB(t)
			proxySvc := &ChainProxyService{}
			hop := mustCreateChainProxy(t, proxySvc, &model.ChainProxy{
				Remark: "hop", Type: "socks", Server: "h.example.com", Port: 1080, Enabled: true,
			})
			selector, err := (&ChainSelectorService{}).Create(&model.ChainSelector{
				Remark: "entries", Strategy: "random", ProxyIds: []int{hop.Id}, Enabled: true,
			})
			if err != nil {
				t.Fatalf("create selector: %v", err)
			}

			ib := makeInboundWithSubSortIndex("in-7101-tcp", 7101, 1)
			if err := database.GetDB().Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}

			proxyId, selectorId := 0, 0
			if tc.proxyId > 0 {
				proxyId = hop.Id
			}
			if tc.selectorId > 0 {
				selectorId = selector.Id
			}
			update := *ib
			update.ChainProxyId = proxyId
			update.ChainSelectorId = selectorId
			if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
				t.Fatalf("UpdateInbound: %v", err)
			}

			var reloaded model.Inbound
			if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
				t.Fatalf("reload: %v", err)
			}
			if reloaded.ChainProxyId != tc.wantProxy || reloaded.ChainSelectorId != tc.wantSelect {
				t.Fatalf("persisted chain = %d/%d, want %d/%d",
					reloaded.ChainProxyId, reloaded.ChainSelectorId, tc.wantProxy, tc.wantSelect)
			}
		})
	}
}

// AddInbound saves the row whole, so an assignment given at create time must
// already be there.
func TestAddInbound_PersistsChainAssignment(t *testing.T) {
	setupChainDB(t)
	hop := mustCreateChainProxy(t, &ChainProxyService{}, &model.ChainProxy{
		Remark: "hop", Type: "socks", Server: "h.example.com", Port: 1080, Enabled: true,
	})

	ib := makeInboundWithSubSortIndex("in-7102-tcp", 7102, 1)
	ib.ChainProxyId = hop.Id
	got, _, err := (&InboundService{}).AddInbound(ib)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, got.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.ChainProxyId != hop.Id {
		t.Fatalf("persisted ChainProxyId = %d, want %d", reloaded.ChainProxyId, hop.Id)
	}
}

// A node holds no chain rows, so the push cannot carry one: the master's own
// assignment has to survive the sync instead of being reset to direct.
func TestUpdateInbound_NodeSyncKeepsChainAssignment(t *testing.T) {
	setupChainDB(t)
	nodeId := 3
	if err := database.GetDB().Create(&model.Node{Id: nodeId, Name: "n3", Address: "n3.example.com"}).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	hop := mustCreateChainProxy(t, &ChainProxyService{}, &model.ChainProxy{
		Remark: "hop", Type: "socks", Server: "h.example.com", Port: 1080, Enabled: true,
	})

	ib := makeInboundWithSubSortIndex("in-7103-tcp", 7103, 1)
	ib.NodeID = &nodeId
	ib.ChainProxyId = hop.Id
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	// A node push re-sends the inbound without any chain field.
	pushed := *ib
	pushed.ChainProxyId = 0
	pushed.ChainSelectorId = 0
	if _, _, err := (&InboundService{FromNodeSync: true}).UpdateInbound(&pushed); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.ChainProxyId != hop.Id {
		t.Fatalf("node sync reset ChainProxyId to %d, want the stored %d", reloaded.ChainProxyId, hop.Id)
	}
}
