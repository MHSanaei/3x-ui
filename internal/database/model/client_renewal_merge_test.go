package model

import "testing"

func TestMergeClientRecordRenewalModes(t *testing.T) {
	for _, tt := range []struct {
		name                string
		existing, incoming  ClientRecord
		reset, day, weekday int
	}{
		{"newer weekly replaces interval", ClientRecord{Reset: 7, UpdatedAt: 1}, ClientRecord{ResetWeekday: 3, UpdatedAt: 2}, 0, 0, 3},
		{"newer weekly replaces legacy monthly", ClientRecord{Reset: 7, ResetDay: 1, UpdatedAt: 1}, ClientRecord{ResetWeekday: 3, UpdatedAt: 2}, 0, 0, 3},
		{"newer interval replaces weekly", ClientRecord{ResetWeekday: 3, UpdatedAt: 1}, ClientRecord{Reset: 7, UpdatedAt: 2}, 7, 0, 0},
		{"newer monthly replaces weekly", ClientRecord{ResetWeekday: 3, UpdatedAt: 1}, ClientRecord{Reset: 7, ResetDay: 1, UpdatedAt: 2}, 7, 1, 0},
		{"older weekly cannot fill interval zeros", ClientRecord{Reset: 7, UpdatedAt: 2}, ClientRecord{ResetWeekday: 3, UpdatedAt: 1}, 7, 0, 0},
		{"older interval cannot fill weekly zeros", ClientRecord{ResetWeekday: 3, UpdatedAt: 2}, ClientRecord{Reset: 7, UpdatedAt: 1}, 0, 0, 3},
		{"older monthly cannot fill weekly zeros", ClientRecord{ResetWeekday: 3, UpdatedAt: 2}, ClientRecord{ResetDay: 1, UpdatedAt: 1}, 0, 0, 3},
		{"empty newer snapshot preserves weekly", ClientRecord{ResetWeekday: 3, UpdatedAt: 1}, ClientRecord{UpdatedAt: 2}, 0, 0, 3},
		{"unset existing accepts weekly", ClientRecord{UpdatedAt: 2}, ClientRecord{ResetWeekday: 3, UpdatedAt: 1}, 0, 0, 3},
		{"legacy monthly merging is unchanged", ClientRecord{Reset: 7, UpdatedAt: 1}, ClientRecord{ResetDay: 1, UpdatedAt: 2}, 7, 1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.existing.ResetMax = 4
			MergeClientRecord(&tt.existing, &tt.incoming)
			if tt.existing.Reset != tt.reset || tt.existing.ResetDay != tt.day || tt.existing.ResetWeekday != tt.weekday || tt.existing.ResetMax != 4 {
				t.Fatalf("merged schedule/cap = %d/%d/%d/%d, want %d/%d/%d/4", tt.existing.Reset, tt.existing.ResetDay, tt.existing.ResetWeekday, tt.existing.ResetMax, tt.reset, tt.day, tt.weekday)
			}
		})
	}
}
