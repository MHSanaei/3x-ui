package service

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestGetXrayConfigScale measures building the full Xray config (the path
// every restart/reconcile takes) and, separately, how much of the per-inbound
// settings rebuild is spent on indented vs plain JSON marshaling.
func TestGetXrayConfigScale(t *testing.T) {
	setupScaleDB(t)
	svc := &XrayService{}
	shapes := []struct {
		name     string
		inbounds int
	}{{"single", 1}, {"spread50", 50}}

	for _, n := range scaleSizes(t, 10000, 100000) {
		for _, shape := range shapes {
			t.Run(fmt.Sprintf("N=%d_%s", n, shape.name), func(t *testing.T) {
				ds := seedScaleDataset(t, n, shape.inbounds)

				const reps = 3
				start := time.Now()
				for range reps {
					cfg, err := svc.GetXrayConfig()
					if err != nil {
						t.Fatalf("GetXrayConfig: %v", err)
					}
					if len(cfg.InboundConfigs) < shape.inbounds {
						t.Fatalf("config has %d inbounds, want >= %d", len(cfg.InboundConfigs), shape.inbounds)
					}
				}
				t.Logf("N=%-7d shape=%-8s GetXrayConfig=%v/run",
					n, shape.name, (time.Since(start) / reps).Round(time.Millisecond))

				var ib model.Inbound
				if err := database.GetDB().First(&ib, ds.inboundIds[0]).Error; err != nil {
					t.Fatalf("load inbound: %v", err)
				}
				settings := map[string]any{}
				if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
					t.Fatalf("unmarshal settings: %v", err)
				}
				start = time.Now()
				if _, err := json.Marshal(settings); err != nil {
					t.Fatalf("marshal settings: %v", err)
				}
				plain := time.Since(start)
				start = time.Now()
				if _, err := json.MarshalIndent(settings, "", "  "); err != nil {
					t.Fatalf("marshal indent settings: %v", err)
				}
				indented := time.Since(start)
				t.Logf("N=%-7d shape=%-8s settingsMarshal plain=%-9v indent=%v",
					n, shape.name, plain.Round(time.Millisecond), indented.Round(time.Millisecond))
			})
		}
	}
}
