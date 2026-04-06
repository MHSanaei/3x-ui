package burst_test

import (
	"math"
	reflect "reflect"
	"testing"
	"time"

	"github.com/xtls/xray-core/app/observatory/burst"
)

func TestHealthPingResults(t *testing.T) {
	rtts := []int64{60, 140, 60, 140, 60, 60, 140, 60, 140}
	hr := burst.NewHealthPingResult(4, time.Hour)
	for _, rtt := range rtts {
		hr.Put(time.Duration(rtt))
	}
	rttFailed := time.Duration(math.MaxInt64)
	expected := &burst.HealthPingStats{
		All:       4,
		Fail:      0,
		Deviation: 40,
		Average:   100,
		Max:       140,
		Min:       60,
	}
	actual := hr.Get()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected: %v, actual: %v", expected, actual)
	}
	hr.Put(rttFailed)
	hr.Put(rttFailed)
	expected.Fail = 2
	actual = hr.Get()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("failed half-failures test, expected: %v, actual: %v", expected, actual)
	}
	hr.Put(rttFailed)
	hr.Put(rttFailed)
	expected = &burst.HealthPingStats{
		All:       4,
		Fail:      4,
		Deviation: 0,
		Average:   0,
		Max:       0,
		Min:       0,
	}
	actual = hr.Get()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("failed all-failures test, expected: %v, actual: %v", expected, actual)
	}
}

func TestHealthPingResultsIgnoreOutdated(t *testing.T) {
	rtts := []int64{60, 140, 60, 140}
	hr := burst.NewHealthPingResult(4, time.Duration(10)*time.Millisecond)
	for i, rtt := range rtts {
		if i == 2 {
			// wait for previous 2 outdated
			time.Sleep(time.Duration(10) * time.Millisecond)
		}
		hr.Put(time.Duration(rtt))
	}
	hr.Get()
	expected := &burst.HealthPingStats{
		All:       2,
		Fail:      0,
		Deviation: 40,
		Average:   100,
		Max:       140,
		Min:       60,
	}
	actual := hr.Get()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("failed 'half-outdated' test, expected: %v, actual: %v", expected, actual)
	}
	// wait for all outdated
	time.Sleep(time.Duration(10) * time.Millisecond)
	expected = &burst.HealthPingStats{
		All:       0,
		Fail:      0,
		Deviation: 0,
		Average:   0,
		Max:       0,
		Min:       0,
	}
	actual = hr.Get()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("failed 'outdated / not-tested' test, expected: %v, actual: %v", expected, actual)
	}

	hr.Put(time.Duration(60))
	expected = &burst.HealthPingStats{
		All:  1,
		Fail: 0,
		// 1 sample, std=0.5rtt
		Deviation: 30,
		Average:   60,
		Max:       60,
		Min:       60,
	}
	actual = hr.Get()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected: %v, actual: %v", expected, actual)
	}
}
