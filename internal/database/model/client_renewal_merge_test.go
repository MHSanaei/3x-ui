package model

import (
	"reflect"
	"testing"
)

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

// A node snapshot must not undo a newer binding, an omitted list must keep the stored one,
// and an explicit empty list clears it.
func TestMergeClientRecordHostBindings(t *testing.T) {
	for _, tt := range []struct {
		name               string
		existing, incoming ClientRecord
		want               []int
	}{
		{"newer list replaces", ClientRecord{ClientHostRuleIds: []int{2}, UpdatedAt: 1}, ClientRecord{ClientHostRuleIds: []int{5, 7}, UpdatedAt: 2}, []int{5, 7}},
		{"older list cannot divert a bound list", ClientRecord{ClientHostRuleIds: []int{2}, UpdatedAt: 2}, ClientRecord{ClientHostRuleIds: []int{9}, UpdatedAt: 1}, []int{2}},
		{"older list fills unset bindings", ClientRecord{UpdatedAt: 2}, ClientRecord{ClientHostRuleIds: []int{9}, UpdatedAt: 1}, []int{9}},
		{"omitted list preserves", ClientRecord{ClientHostRuleIds: []int{2}, UpdatedAt: 1}, ClientRecord{UpdatedAt: 2}, []int{2}},
		{"newer empty list clears", ClientRecord{ClientHostRuleIds: []int{2}, UpdatedAt: 1}, ClientRecord{ClientHostRuleIds: []int{}, UpdatedAt: 2}, []int{}},
		{"merged list is normalized", ClientRecord{UpdatedAt: 1}, ClientRecord{ClientHostRuleIds: []int{3, 0, 3}, UpdatedAt: 2}, []int{3}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			MergeClientRecord(&tt.existing, &tt.incoming)
			if got := tt.existing.ClientHostRuleIds; got == nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ClientHostRuleIds = %#v, want %#v", got, tt.want)
			}
		})
	}
}
