package model

import (
	"reflect"
	"testing"
)

func intPtr(v int) *int { return &v }

func TestNormalizeClientHostBindings(t *testing.T) {
	for _, tt := range []struct {
		name        string
		single      *int
		list        []int
		wantSingle  *int
		wantList    []int
		wantListNil bool
	}{
		{"nil preserves", nil, nil, nil, nil, true},
		{"empty clears", nil, []int{}, nil, []int{}, false},
		{"single-only migrates", intPtr(5), nil, intPtr(5), []int{5}, false},
		{"list-only mirrors first", nil, []int{2, 3}, intPtr(2), []int{2, 3}, false},
		{"conflict list wins", intPtr(9), []int{2, 3}, intPtr(2), []int{2, 3}, false},
		{"dedupe keeps order", nil, []int{3, 2, 3, 2}, intPtr(3), []int{3, 2}, false},
		{"non-positive dropped", nil, []int{0, -1, 2, 0, 3}, intPtr(2), []int{2, 3}, false},
		{"all non-positive clears", nil, []int{0, -2}, nil, []int{}, false},
		{"single non-positive normalizes", intPtr(0), nil, nil, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			single := tt.single
			var list []int
			if tt.list != nil {
				list = append([]int(nil), tt.list...)
				if len(tt.list) == 0 {
					list = []int{}
				}
			}
			NormalizeClientHostBindings(&single, &list)
			if tt.wantListNil {
				if list != nil {
					t.Fatalf("list = %v, want nil", list)
				}
			} else {
				if list == nil {
					t.Fatalf("list = nil, want %v", tt.wantList)
				}
				if !reflect.DeepEqual(list, tt.wantList) {
					t.Fatalf("list = %v, want %v", list, tt.wantList)
				}
			}
			if tt.wantSingle == nil {
				if single != nil {
					t.Fatalf("single = %v, want nil", *single)
				}
			} else {
				if single == nil {
					t.Fatalf("single = nil, want %d", *tt.wantSingle)
				}
				if *single != *tt.wantSingle {
					t.Fatalf("single = %d, want %d", *single, *tt.wantSingle)
				}
			}
		})
	}
}
