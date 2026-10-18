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

// Host bindings are one logical field behind two encodings (canonical list,
// legacy alias); otherwise the alias and the list diverge and the reader
// unions both into phantom bindings.
func TestMergeClientRecordHostBindings(t *testing.T) {
	for _, tt := range []struct {
		name               string
		existing, incoming ClientRecord
		wantSingle         *int
		wantList           []int
		wantListNil        bool
	}{
		{
			"newer alias-only rebinding replaces the set",
			ClientRecord{ClientHostRuleId: intPtr(2), ClientHostRuleIds: []int{2}, UpdatedAt: 1},
			ClientRecord{ClientHostRuleId: intPtr(9), UpdatedAt: 2},
			intPtr(9),
			[]int{9},
			false,
		},
		{
			"newer alias-only rebinding on list-bound client",
			ClientRecord{ClientHostRuleIds: []int{2}, UpdatedAt: 1},
			ClientRecord{ClientHostRuleId: intPtr(9), UpdatedAt: 2},
			intPtr(9),
			[]int{9},
			false,
		},
		{
			"older alias cannot divert a bound list",
			ClientRecord{ClientHostRuleIds: []int{2}, UpdatedAt: 2},
			ClientRecord{ClientHostRuleId: intPtr(9), UpdatedAt: 1},
			intPtr(2),
			[]int{2},
			false,
		},
		{
			"older alias-only migrates unset bindings",
			ClientRecord{UpdatedAt: 2},
			ClientRecord{ClientHostRuleId: intPtr(9), UpdatedAt: 1},
			intPtr(9),
			[]int{9},
			false,
		},
		{
			"newer explicit list replaces",
			ClientRecord{ClientHostRuleId: intPtr(2), ClientHostRuleIds: []int{2}, UpdatedAt: 1},
			ClientRecord{ClientHostRuleId: intPtr(5), ClientHostRuleIds: []int{5, 7}, UpdatedAt: 2},
			intPtr(5),
			[]int{5, 7},
			false,
		},
		{
			"empty snapshot preserves bindings",
			ClientRecord{ClientHostRuleId: intPtr(2), ClientHostRuleIds: []int{2, 3}, UpdatedAt: 1},
			ClientRecord{UpdatedAt: 2},
			intPtr(2),
			[]int{2, 3},
			false,
		},
		{
			"explicit empty list clears",
			ClientRecord{ClientHostRuleId: intPtr(2), ClientHostRuleIds: []int{2}, UpdatedAt: 1},
			ClientRecord{ClientHostRuleIds: []int{}, UpdatedAt: 2},
			nil,
			[]int{},
			false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.existing.ClientHostRuleIds != nil {
				tt.existing.ClientHostRuleIds = CloneClientHostRuleIds(tt.existing.ClientHostRuleIds)
				if len(tt.existing.ClientHostRuleIds) == 0 {
					tt.existing.ClientHostRuleIds = []int{}
				}
			}
			if tt.incoming.ClientHostRuleIds != nil {
				tt.incoming.ClientHostRuleIds = CloneClientHostRuleIds(tt.incoming.ClientHostRuleIds)
				if len(tt.incoming.ClientHostRuleIds) == 0 {
					tt.incoming.ClientHostRuleIds = []int{}
				}
			}
			MergeClientRecord(&tt.existing, &tt.incoming)
			got := tt.existing
			if tt.wantListNil {
				if got.ClientHostRuleIds != nil {
					t.Fatalf("list = %v, want nil", got.ClientHostRuleIds)
				}
			} else {
				if got.ClientHostRuleIds == nil {
					t.Fatalf("list = nil, want %v", tt.wantList)
				}
				if !reflect.DeepEqual(got.ClientHostRuleIds, tt.wantList) {
					t.Fatalf("list = %v, want %v", got.ClientHostRuleIds, tt.wantList)
				}
			}
			if tt.wantSingle == nil {
				if got.ClientHostRuleId != nil {
					t.Fatalf("single = %v, want nil", *got.ClientHostRuleId)
				}
			} else {
				if got.ClientHostRuleId == nil {
					t.Fatalf("single = nil, want %d", *tt.wantSingle)
				}
				if *got.ClientHostRuleId != *tt.wantSingle {
					t.Fatalf("single = %d, want %d", *got.ClientHostRuleId, *tt.wantSingle)
				}
			}
		})
	}
}
