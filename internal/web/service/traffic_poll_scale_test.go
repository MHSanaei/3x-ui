package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func pollReport(ds scaleDataset, k int) ([]*xray.Traffic, []*xray.ClientTraffic) {
	traffics := make([]*xray.Traffic, 0, len(ds.tags))
	for _, tag := range ds.tags {
		traffics = append(traffics, &xray.Traffic{IsInbound: true, Tag: tag, Up: 1 << 20, Down: 2 << 20})
	}
	clientTraffics := make([]*xray.ClientTraffic, 0, k)
	for _, email := range sampleEmails(ds.emails, k) {
		clientTraffics = append(clientTraffics, &xray.ClientTraffic{Email: email, Up: 1 << 20, Down: 2 << 20})
	}
	return traffics, clientTraffics
}

// TestAddTrafficPollScale measures one full traffic-poll cycle (the @every 5s
// job): per-client delta UPDATEs, the auto-renew probe and the depleted-client
// scan, in steady state and with 100 clients depleting / renewing.
func TestAddTrafficPollScale(t *testing.T) {
	setupScaleDB(t)
	svc := &InboundService{}
	shapes := []struct {
		name     string
		inbounds int
	}{{"single", 1}, {"spread50", 50}}

	for _, n := range scaleSizes(t, 10000, 100000) {
		for _, shape := range shapes {
			t.Run(fmt.Sprintf("N=%d_%s", n, shape.name), func(t *testing.T) {
				db := database.GetDB()
				ds := seedScaleDataset(t, n, shape.inbounds)

				for _, k := range []int{1000, 10000} {
					if k > n {
						continue
					}
					traffics, clientTraffics := pollReport(ds, k)
					const reps = 3
					start := time.Now()
					for range reps {
						if _, _, err := svc.AddTraffic(traffics, clientTraffics); err != nil {
							t.Fatalf("AddTraffic steady: %v", err)
						}
					}
					perPoll := time.Since(start) / reps
					t.Logf("N=%-7d shape=%-8s K=%-6d steady=%v/poll", n, shape.name, k, perPoll.Round(time.Millisecond))

					var probe xray.ClientTraffic
					if err := db.Where("email = ?", clientTraffics[0].Email).First(&probe).Error; err != nil {
						t.Fatalf("load probe row: %v", err)
					}
					if probe.Up == 0 || probe.Down == 0 {
						t.Fatalf("steady polls did not accumulate traffic: up=%d down=%d", probe.Up, probe.Down)
					}
				}

				depleted := ds.perInbound[0][:100]
				if err := db.Model(&xray.ClientTraffic{}).
					Where("email IN ?", emailsOf(depleted)).
					Updates(map[string]any{"up": int64(100 << 30), "down": int64(0)}).Error; err != nil {
					t.Fatalf("mark depleted: %v", err)
				}
				start := time.Now()
				if _, _, err := svc.AddTraffic(nil, nil); err != nil {
					t.Fatalf("AddTraffic disable: %v", err)
				}
				t.Logf("N=%-7d shape=%-8s disable100=%v", n, shape.name, time.Since(start).Round(time.Millisecond))
				var disabledCount int64
				if err := db.Model(&xray.ClientTraffic{}).Where("enable = ?", false).Count(&disabledCount).Error; err != nil {
					t.Fatalf("count disabled: %v", err)
				}
				if disabledCount != 100 {
					t.Fatalf("disable100: got %d disabled rows, want 100", disabledCount)
				}

				renew := ds.perInbound[0][100:200]
				past := time.Now().Add(-time.Hour).UnixMilli()
				if err := db.Model(&xray.ClientTraffic{}).
					Where("email IN ?", emailsOf(renew)).
					Updates(map[string]any{"reset": 30, "expiry_time": past, "up": int64(1 << 30)}).Error; err != nil {
					t.Fatalf("mark renewable: %v", err)
				}
				start = time.Now()
				if _, _, err := svc.AddTraffic(nil, nil); err != nil {
					t.Fatalf("AddTraffic renew: %v", err)
				}
				t.Logf("N=%-7d shape=%-8s renew100=%v", n, shape.name, time.Since(start).Round(time.Millisecond))
				var renewed xray.ClientTraffic
				if err := db.Where("email = ?", renew[0].Email).First(&renewed).Error; err != nil {
					t.Fatalf("load renewed row: %v", err)
				}
				if renewed.ExpiryTime <= past || renewed.Up != 0 {
					t.Fatalf("renew100 did not renew: expiry=%d up=%d", renewed.ExpiryTime, renewed.Up)
				}
			})
		}
	}
}
