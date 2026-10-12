package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A node detach keeps the client's stat row (keepTraffic), so the node goes on
// reporting it under an inbound whose settings no longer list the client (#6724).
func TestNodeSnapshotIgnoresStatsOfClientDetachedFromInbound(t *testing.T) {
	const gib = int64(1) << 30
	const emptyClients = `{"clients": []}`

	t.Run("stale node quota and disable never reach the master row", func(t *testing.T) {
		db := initTrafficTestDB(t)
		createNodeInbound(t, db, 1, "n1-in", 41001)
		local := &model.Inbound{UserId: 1, Tag: "local-in", Enable: true, Port: 41010, Protocol: model.VLESS}
		if err := db.Create(local).Error; err != nil {
			t.Fatalf("create local inbound: %v", err)
		}
		const email = "moved"
		if err := db.Create(&xray.ClientTraffic{InboundId: local.Id, Email: email, Enable: true, Total: 200 * gib, Up: 5, Down: 5}).Error; err != nil {
			t.Fatalf("seed client_traffics: %v", err)
		}
		if err := db.Create(&model.NodeClientTraffic{NodeId: 1, Email: email, Up: 40, Down: 40}).Error; err != nil {
			t.Fatalf("seed node baseline: %v", err)
		}

		svc := &InboundService{}
		// Two ticks: the first copies the quota, which makes the second's disable look genuine.
		for range 2 {
			syncNodeWithSettings(t, svc, 1, "n1-in", emptyClients,
				xray.ClientTraffic{Email: email, Up: 40, Down: 40, Total: 100 * gib, Enable: false})
		}

		got := readTraffic(t, db, email)
		if got.Total != 200*gib || !got.Enable {
			t.Fatalf("master row = total %d enable %v, want total %d enable true", got.Total, got.Enable, 200*gib)
		}
	})

	t.Run("a detached email gets no master traffic row", func(t *testing.T) {
		db := initTrafficTestDB(t)
		createNodeInbound(t, db, 1, "n1-in", 41001)

		svc := &InboundService{}
		syncNodeWithSettings(t, svc, 1, "n1-in", emptyClients,
			xray.ClientTraffic{Email: "gone", Up: 40, Down: 40, Total: 100 * gib, Enable: true})

		var ct xray.ClientTraffic
		err := db.Model(xray.ClientTraffic{}).Where("email = ?", "gone").First(&ct).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("client_traffics row for detached email: err = %v, row = %+v; want ErrRecordNotFound", err, ct)
		}
	})
}
