package service

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestWsPayloadScale compares the websocket broadcast payload paths the
// traffic job runs every 5s while a browser is connected: the full snapshot
// (GetAllClientTraffics + full last-online map) against the delta variant
// (GetActiveClientTraffics on this poll's active emails). Payload sizes are
// logged against the hub's 10MB drop threshold.
func TestWsPayloadScale(t *testing.T) {
	setupScaleDB(t)
	svc := &InboundService{}
	const hubPayloadLimit = 10 * 1024 * 1024

	for _, n := range scaleSizes(t, 10000, 100000) {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			ds := seedScaleDataset(t, n, 1)
			k := min(10000, n)
			activeEmails := sampleEmails(ds.emails, k)

			start := time.Now()
			all, err := svc.GetAllClientTraffics()
			if err != nil {
				t.Fatalf("GetAllClientTraffics: %v", err)
			}
			fetchAll := time.Since(start)
			if len(all) != n {
				t.Fatalf("GetAllClientTraffics rows = %d, want %d", len(all), n)
			}
			start = time.Now()
			snapshot, err := json.Marshal(map[string]any{"clients": all})
			if err != nil {
				t.Fatalf("marshal snapshot: %v", err)
			}
			marshalAll := time.Since(start)
			verdict := "delivered"
			if len(snapshot) > hubPayloadLimit {
				verdict = "DROPPED by hub (>10MB)"
			}
			t.Logf("N=%-7d snapshot: fetch=%-9v marshal=%-9v payload=%.1fMB %s",
				n, fetchAll.Round(time.Millisecond), marshalAll.Round(time.Millisecond),
				float64(len(snapshot))/(1<<20), verdict)

			start = time.Now()
			active, err := svc.GetActiveClientTraffics(activeEmails)
			if err != nil {
				t.Fatalf("GetActiveClientTraffics: %v", err)
			}
			fetchActive := time.Since(start)
			if len(active) != k {
				t.Fatalf("GetActiveClientTraffics rows = %d, want %d", len(active), k)
			}
			start = time.Now()
			delta, err := json.Marshal(map[string]any{"clients": active})
			if err != nil {
				t.Fatalf("marshal delta: %v", err)
			}
			marshalActive := time.Since(start)
			t.Logf("N=%-7d delta(K=%d): fetch=%-9v marshal=%-9v payload=%.1fMB",
				n, k, fetchActive.Round(time.Millisecond), marshalActive.Round(time.Millisecond),
				float64(len(delta))/(1<<20))

			start = time.Now()
			lastOnline, err := svc.GetClientsLastOnline()
			if err != nil {
				t.Fatalf("GetClientsLastOnline: %v", err)
			}
			fullMap := time.Since(start)
			if len(lastOnline) != n {
				t.Fatalf("GetClientsLastOnline entries = %d, want %d", len(lastOnline), n)
			}
			start = time.Now()
			activeLastOnline := make(map[string]int64, len(active))
			for _, ct := range active {
				activeLastOnline[ct.Email] = ct.LastOnline
			}
			activeMap := time.Since(start)
			t.Logf("N=%-7d lastOnline: fullMap=%-9v activeMap(K=%d)=%v",
				n, fullMap.Round(time.Millisecond), k, activeMap.Round(time.Microsecond))

			start = time.Now()
			if _, err := svc.GetInboundsTrafficSummary(); err != nil {
				t.Fatalf("GetInboundsTrafficSummary: %v", err)
			}
			t.Logf("N=%-7d inboundsSummary=%v", n, time.Since(start).Round(time.Millisecond))
		})
	}
}
