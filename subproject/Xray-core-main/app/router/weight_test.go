package router_test

import (
	"reflect"
	"testing"

	"github.com/xtls/xray-core/app/router"
)

func TestWeight(t *testing.T) {
	manager := router.NewWeightManager(
		[]*router.StrategyWeight{
			{
				Match: "x5",
				Value: 100,
			},
			{
				Match: "x8",
			},
			{
				Regexp: true,
				Match:  `\bx0+(\.\d+)?\b`,
				Value:  1,
			},
			{
				Regexp: true,
				Match:  `\bx\d+(\.\d+)?\b`,
			},
		},
		1, func(v, w float64) float64 {
			return v * w
		},
	)
	tags := []string{
		"node name, x5, and more",
		"node name, x8",
		"node name, x15",
		"node name, x0100, and more",
		"node name, x10.1",
		"node name, x00.1, and more",
	}
	// test weight
	expected := []float64{100, 8, 15, 100, 10.1, 1}
	actual := make([]float64, 0)
	for _, tag := range tags {
		actual = append(actual, manager.Get(tag))
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected: %v, actual: %v", expected, actual)
	}
	// test scale
	expected2 := []float64{1000, 80, 150, 1000, 101, 10}
	actual2 := make([]float64, 0)
	for _, tag := range tags {
		actual2 = append(actual2, manager.Apply(tag, 10))
	}
	if !reflect.DeepEqual(expected2, actual2) {
		t.Errorf("expected2: %v, actual2: %v", expected2, actual2)
	}
}
