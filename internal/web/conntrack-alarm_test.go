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
		name    string
		enabled bool
		events  string
		want    bool
	}{
		{"bot disabled", false, "conntrack.high", false},
		{"unselected", true, "cpu.high,memory.high", false},
		{"high", true, "cpu.high, conntrack.high ", true},
		{"recovery", true, "conntrack.recovered", true},
		{"empty", true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.settingService.SetTgbotEnabled(tc.enabled); err != nil {
				t.Fatal(err)
			}
			if err := s.settingService.SetTgEnabledEvents(tc.events); err != nil {
				t.Fatal(err)
			}
			if got := s.conntrackAlarmWanted(); got != tc.want {
				t.Fatalf("conntrackAlarmWanted() = %v, want %v", got, tc.want)
			}
		})
	}
}
