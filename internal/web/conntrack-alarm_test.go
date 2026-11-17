package web

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

func TestConntrackAlarmWanted(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "panel.db"))
	s := &Server{}
	for _, tc := range []struct {
		name      string
		enabled   bool
		events    string
		threshold int
		want      bool
	}{
		{"bot disabled", false, "conntrack.high", 80, false},
		{"unselected", true, "cpu.high,memory.high", 80, false},
		{"high", true, "cpu.high, conntrack.high ", 80, true},
		{"custom threshold", true, "conntrack.high", 55, true},
		{"zero threshold", true, "conntrack.high", 0, false},
		{"removed recovery", true, "conntrack.recovered", 80, false},
		{"empty", true, "", 80, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.settingService.SetTgbotEnabled(tc.enabled); err != nil {
				t.Fatal(err)
			}
			if err := s.settingService.SetTgEnabledEvents(tc.events); err != nil {
				t.Fatal(err)
			}
			if err := s.settingService.SetTgConntrack(tc.threshold); err != nil {
				t.Fatal(err)
			}
			if got := s.conntrackAlarmWanted(); got != tc.want {
				t.Fatalf("conntrackAlarmWanted() = %v, want %v", got, tc.want)
			}
		})
	}
}
