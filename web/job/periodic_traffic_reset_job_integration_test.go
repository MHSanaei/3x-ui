package job

import (
	"fmt"
	"testing"
	"time"

	"x-ui/database/model"
)

// Tests to ensure the job handles various real-world scenarios properly
func TestPeriodicTrafficResetJobNewInstance(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	if job == nil {
		t.Fatal("Expected job to be created, got nil")
	}

	if job.lastResetTimes == nil {
		t.Fatal("Expected lastResetTimes map to be initialized")
	}

	if len(job.lastResetTimes) != 0 {
		t.Error("Expected lastResetTimes map to be empty initially")
	}
}

func TestPeriodicTrafficResetJobBoundaryConditions(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	// Test edge cases for time boundaries
	testCases := []struct {
		name           string
		resetType      string
		now            time.Time
		lastReset      time.Time
		expectedResult bool
	}{
		{
			name:           "daily reset at exact midnight boundary",
			resetType:      "daily",
			now:            time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			lastReset:      time.Time{},
			expectedResult: true,
		},
		{
			name:           "daily reset one second after midnight boundary",
			resetType:      "daily",
			now:            time.Date(2024, 1, 15, 0, 0, 1, 0, time.UTC),
			lastReset:      time.Time{},
			expectedResult: true,
		},
		{
			name:           "weekly reset on Sunday at exact midnight",
			resetType:      "weekly",
			now:            time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC), // Sunday
			lastReset:      time.Time{},
			expectedResult: true,
		},
		{
			name:           "monthly reset on 1st at exact midnight",
			resetType:      "monthly",
			now:            time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			lastReset:      time.Time{},
			expectedResult: true,
		},
		{
			name:           "daily reset at 10-minute boundary (should not reset)",
			resetType:      "daily",
			now:            time.Date(2024, 1, 15, 0, 10, 0, 0, time.UTC),
			lastReset:      time.Time{},
			expectedResult: false,
		},
		{
			name:           "weekly reset at 10-minute boundary (should not reset)",
			resetType:      "weekly",
			now:            time.Date(2024, 1, 14, 0, 10, 0, 0, time.UTC), // Sunday
			lastReset:      time.Time{},
			expectedResult: false,
		},
		{
			name:           "monthly reset at 10-minute boundary (should not reset)",
			resetType:      "monthly",
			now:            time.Date(2024, 2, 1, 0, 10, 0, 0, time.UTC),
			lastReset:      time.Time{},
			expectedResult: false,
		},
		{
			name:           "leap year handling - February 29th to March 1st",
			resetType:      "monthly",
			now:            time.Date(2024, 3, 1, 0, 5, 0, 0, time.UTC),
			lastReset:      time.Date(2024, 2, 1, 0, 5, 0, 0, time.UTC), // 29 days ago
			expectedResult: true,
		},
		{
			name:           "weekly reset after 6 days and 23 hours (should not reset)",
			resetType:      "weekly",
			now:            time.Date(2024, 1, 20, 23, 0, 0, 0, time.UTC), // Saturday
			lastReset:      time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC), // Previous Sunday
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{
				Id:                   1,
				Tag:                  "test-inbound",
				PeriodicTrafficReset: tc.resetType,
			}

			// Set up last reset time if provided
			if !tc.lastReset.IsZero() {
				job.updateLastResetTime(inbound, tc.lastReset)
			}

			result := job.shouldResetTraffic(inbound, tc.now)
			if result != tc.expectedResult {
				t.Errorf("Expected %v, got %v", tc.expectedResult, result)
			}

			// Clean up for next test
			job.lastResetTimes = make(map[string]time.Time)
		})
	}
}

func TestPeriodicTrafficResetJobTimezoneHandling(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	inbound := &model.Inbound{
		Id:                   1,
		Tag:                  "timezone-test",
		PeriodicTrafficReset: "daily",
	}

	// Test that UTC midnight behaves consistently
	utcMidnight := time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC)
	result := job.shouldResetTraffic(inbound, utcMidnight)
	
	if !result {
		t.Error("Expected reset at UTC midnight")
	}

	// Test that the same instant in different timezones produces the same absolute behavior
	// This tests consistency rather than timezone conversion
	utcTime := time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC)
	utcTimestamp := utcTime.Unix()
	
	// Create equivalent time from timestamp
	equivalentTime := time.Unix(utcTimestamp, 0).UTC()
	
	result1 := job.shouldResetTraffic(inbound, utcTime)
	result2 := job.shouldResetTraffic(inbound, equivalentTime)
	
	if result1 != result2 {
		t.Errorf("Equivalent times should behave consistently: %v vs %v", result1, result2)
	}
}

func TestPeriodicTrafficResetJobConcurrentSafety(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	inbound := &model.Inbound{
		Id:                   1,
		Tag:                  "concurrent-test",
		PeriodicTrafficReset: "daily",
	}

	// Test concurrent operations
	done := make(chan bool, 100)
	
	// Start multiple goroutines performing different operations
	for i := 0; i < 50; i++ {
		go func(i int) {
			defer func() { done <- true }()
			
			// Mix of operations
			testTime := time.Date(2024, 1, 15+i%5, 0, 5, 0, 0, time.UTC)
			job.updateLastResetTime(inbound, testTime)
			_ = job.getLastResetTime(job.getResetKey(inbound))
			_ = job.shouldResetTraffic(inbound, testTime)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 50; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timed out - possible deadlock")
		}
	}
}

func TestPeriodicTrafficResetJobValidationRobustness(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	testCases := []struct {
		name     string
		inbound  *model.Inbound
		now      time.Time
		expected bool
	}{
		{
			name: "nil inbound",
			inbound: nil,
			now:  time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			// This should panic or handle gracefully - depends on implementation
		},
		{
			name: "empty tag",
			inbound: &model.Inbound{
				Id:                   1,
				Tag:                  "",
				PeriodicTrafficReset: "daily",
			},
			now:      time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected: true, // Should still work with empty tag
		},
		{
			name: "very long tag",
			inbound: &model.Inbound{
				Id:                   1,
				Tag:                  "very-very-very-long-tag-name-that-might-cause-issues-with-memory-or-key-generation-in-the-reset-tracking-system",
				PeriodicTrafficReset: "daily",
			},
			now:      time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "special characters in tag",
			inbound: &model.Inbound{
				Id:                   1,
				Tag:                  "test-inbound_with.special@chars#123",
				PeriodicTrafficReset: "daily",
			},
			now:      time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "zero time",
			inbound: &model.Inbound{
				Id:                   1,
				Tag:                  "zero-time-test",
				PeriodicTrafficReset: "daily",
			},
			now:      time.Time{}, // Zero time
			expected: true,        // Zero time has hour=0, minute=0, so should reset according to logic
		},
		{
			name: "far future time",
			inbound: &model.Inbound{
				Id:                   1,
				Tag:                  "future-test",
				PeriodicTrafficReset: "daily",
			},
			now:      time.Date(2099, 12, 31, 0, 5, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "far past time",
			inbound: &model.Inbound{
				Id:                   1,
				Tag:                  "past-test",
				PeriodicTrafficReset: "daily",
			},
			now:      time.Date(1970, 1, 1, 0, 5, 0, 0, time.UTC),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if tc.inbound == nil {
						// Expected panic for nil inbound
						t.Log("Expected panic for nil inbound:", r)
					} else {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()

			if tc.inbound == nil {
				// Skip execution for nil test case
				return
			}

			result := job.shouldResetTraffic(tc.inbound, tc.now)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestPeriodicTrafficResetJobMemoryManagement(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	// Test with many inbounds to check memory usage
	const numInbounds = 1000
	testTime := time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC)

	for i := 0; i < numInbounds; i++ {
		inbound := &model.Inbound{
			Id:                   i,
			Tag:                  fmt.Sprintf("test-inbound-%d", i),
			PeriodicTrafficReset: "daily",
		}
		job.updateLastResetTime(inbound, testTime)
	}

	// Verify all entries are stored
	if len(job.lastResetTimes) != numInbounds {
		t.Errorf("Expected %d entries in lastResetTimes, got %d", numInbounds, len(job.lastResetTimes))
	}

	// Test retrieval of all entries
	for i := 0; i < numInbounds; i++ {
		inbound := &model.Inbound{
			Id:                   i,
			Tag:                  fmt.Sprintf("test-inbound-%d", i),
			PeriodicTrafficReset: "daily",
		}
		retrievedTime := job.getLastResetTime(job.getResetKey(inbound))
		if !retrievedTime.Equal(testTime) {
			t.Errorf("Expected time %v for inbound %d, got %v", testTime, i, retrievedTime)
		}
	}
}
