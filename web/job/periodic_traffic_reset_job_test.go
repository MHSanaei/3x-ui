package job

import (
	"testing"
	"time"

	"x-ui/database/model"
)

func TestShouldResetDaily(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	tests := []struct {
		name      string
		now       time.Time
		lastReset time.Time
		expected  bool
	}{
		{
			name:      "first time reset at midnight",
			now:       time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			lastReset: time.Time{}, // zero time
			expected:  true,
		},
		{
			name:      "first time reset not at midnight",
			now:       time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			lastReset: time.Time{}, // zero time
			expected:  false,
		},
		{
			name:      "reset after 24 hours at midnight",
			now:       time.Date(2024, 1, 16, 0, 5, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected:  true,
		},
		{
			name:      "reset after 24 hours but not at midnight",
			now:       time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "reset same day",
			now:       time.Date(2024, 1, 15, 0, 8, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "reset after 23 hours",
			now:       time.Date(2024, 1, 15, 23, 5, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "midnight but minute > 10",
			now:       time.Date(2024, 1, 16, 0, 15, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "edge case: exactly at minute 10",
			now:       time.Date(2024, 1, 16, 0, 10, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "edge case: minute 9 (last valid minute)",
			now:       time.Date(2024, 1, 16, 0, 9, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := job.shouldResetDaily(tt.now, tt.lastReset)
			if result != tt.expected {
				t.Errorf("shouldResetDaily() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestShouldResetWeekly(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	tests := []struct {
		name      string
		now       time.Time
		lastReset time.Time
		expected  bool
	}{
		{
			name:      "first time reset on Sunday at midnight",
			now:       time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC), // Sunday
			lastReset: time.Time{}, // zero time
			expected:  true,
		},
		{
			name:      "first time reset on Monday",
			now:       time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC), // Monday
			lastReset: time.Time{}, // zero time
			expected:  false,
		},
		{
			name:      "reset after 7 days on Sunday at midnight",
			now:       time.Date(2024, 1, 21, 0, 5, 0, 0, time.UTC), // Sunday
			lastReset: time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC), // Previous Sunday
			expected:  true,
		},
		{
			name:      "reset after 7 days on Sunday but not at midnight",
			now:       time.Date(2024, 1, 21, 12, 0, 0, 0, time.UTC), // Sunday
			lastReset: time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "reset after 6 days on Saturday",
			now:       time.Date(2024, 1, 20, 0, 5, 0, 0, time.UTC), // Saturday
			lastReset: time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "reset same Sunday",
			now:       time.Date(2024, 1, 14, 0, 8, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "reset after 8 days but on Monday",
			now:       time.Date(2024, 1, 22, 0, 5, 0, 0, time.UTC), // Monday
			lastReset: time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC), // Previous Sunday
			expected:  false,
		},
		{
			name:      "edge case: Sunday but minute > 10",
			now:       time.Date(2024, 1, 21, 0, 15, 0, 0, time.UTC), // Sunday
			lastReset: time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := job.shouldResetWeekly(tt.now, tt.lastReset)
			if result != tt.expected {
				t.Errorf("shouldResetWeekly() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestShouldResetMonthly(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	tests := []struct {
		name      string
		now       time.Time
		lastReset time.Time
		expected  bool
	}{
		{
			name:      "first time reset on 1st day at midnight",
			now:       time.Date(2024, 2, 1, 0, 5, 0, 0, time.UTC),
			lastReset: time.Time{}, // zero time
			expected:  true,
		},
		{
			name:      "first time reset on 15th day",
			now:       time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			lastReset: time.Time{}, // zero time
			expected:  false,
		},
		{
			name:      "reset after 28 days on 1st day at midnight",
			now:       time.Date(2024, 2, 1, 0, 5, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 4, 0, 5, 0, 0, time.UTC), // 28 days ago
			expected:  true,
		},
		{
			name:      "reset after 32 days on 1st day at midnight",
			now:       time.Date(2024, 3, 1, 0, 5, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 29, 0, 5, 0, 0, time.UTC), // 32 days ago
			expected:  true,
		},
		{
			name:      "reset after 28 days on 1st day but not at midnight",
			now:       time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 4, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "reset after 27 days",
			now:       time.Date(2024, 1, 31, 0, 5, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 4, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "reset same day (1st)",
			now:       time.Date(2024, 1, 1, 0, 8, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "reset after 28 days but on 2nd day",
			now:       time.Date(2024, 2, 2, 0, 5, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 5, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "leap year February edge case",
			now:       time.Date(2024, 3, 1, 0, 5, 0, 0, time.UTC), // March 1st in leap year
			lastReset: time.Date(2024, 2, 1, 0, 5, 0, 0, time.UTC), // February 1st (29 days ago)
			expected:  true,
		},
		{
			name:      "edge case: 1st day but minute > 10",
			now:       time.Date(2024, 2, 1, 0, 15, 0, 0, time.UTC),
			lastReset: time.Date(2024, 1, 4, 0, 5, 0, 0, time.UTC),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := job.shouldResetMonthly(tt.now, tt.lastReset)
			if result != tt.expected {
				t.Errorf("shouldResetMonthly() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestShouldResetTraffic(t *testing.T) {
	tests := []struct {
		name      string
		inbound   *model.Inbound
		now       time.Time
		setup     func(*PeriodicTrafficResetJob, *model.Inbound)
		expected  bool
	}{
		{
			name: "never reset type",
			inbound: &model.Inbound{
				Id:                   1,
				Tag:                  "test1",
				PeriodicTrafficReset: "never",
			},
			now:      time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected: false,
		},
		{
			name: "invalid reset type",
			inbound: &model.Inbound{
				Id:                   2,
				Tag:                  "test2",
				PeriodicTrafficReset: "invalid",
			},
			now:      time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected: false,
		},
		{
			name: "daily reset should trigger",
			inbound: &model.Inbound{
				Id:                   3,
				Tag:                  "test3",
				PeriodicTrafficReset: "daily",
			},
			now:      time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "weekly reset should trigger",
			inbound: &model.Inbound{
				Id:                   4,
				Tag:                  "test4",
				PeriodicTrafficReset: "weekly",
			},
			now:      time.Date(2024, 1, 14, 0, 5, 0, 0, time.UTC), // Sunday
			expected: true,
		},
		{
			name: "monthly reset should trigger",
			inbound: &model.Inbound{
				Id:                   5,
				Tag:                  "test5",
				PeriodicTrafficReset: "monthly",
			},
			now:      time.Date(2024, 2, 1, 0, 5, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "daily reset with existing reset time - should not trigger",
			inbound: &model.Inbound{
				Id:                   6,
				Tag:                  "test6",
				PeriodicTrafficReset: "daily",
			},
			now: time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC),
			setup: func(j *PeriodicTrafficResetJob, inbound *model.Inbound) {
				j.updateLastResetTime(inbound, time.Date(2024, 1, 15, 0, 3, 0, 0, time.UTC))
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new job for each test to avoid state interference
			testJob := NewPeriodicTrafficResetJob()
			
			if tt.setup != nil {
				tt.setup(testJob, tt.inbound)
			}

			result := testJob.shouldResetTraffic(tt.inbound, tt.now)
			if result != tt.expected {
				t.Errorf("shouldResetTraffic() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetResetKey(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	tests := []struct {
		name     string
		inbound  *model.Inbound
		expected string
	}{
		{
			name: "daily reset key",
			inbound: &model.Inbound{
				Tag:                  "test-inbound",
				PeriodicTrafficReset: "daily",
			},
			expected: "daily_test-inbound",
		},
		{
			name: "weekly reset key",
			inbound: &model.Inbound{
				Tag:                  "proxy-1",
				PeriodicTrafficReset: "weekly",
			},
			expected: "weekly_proxy-1",
		},
		{
			name: "monthly reset key",
			inbound: &model.Inbound{
				Tag:                  "vless-tcp",
				PeriodicTrafficReset: "monthly",
			},
			expected: "monthly_vless-tcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := job.getResetKey(tt.inbound)
			if result != tt.expected {
				t.Errorf("getResetKey() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestLastResetTimeOperations(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	inbound := &model.Inbound{
		Id:                   1,
		Tag:                  "test-inbound",
		PeriodicTrafficReset: "daily",
	}

	// Test getting non-existent reset time (should return zero time)
	resetTime := job.getLastResetTime(job.getResetKey(inbound))
	if !resetTime.IsZero() {
		t.Errorf("Expected zero time for non-existent key, got %v", resetTime)
	}

	// Test updating and getting reset time
	testTime := time.Date(2024, 1, 15, 0, 5, 0, 0, time.UTC)
	job.updateLastResetTime(inbound, testTime)

	retrievedTime := job.getLastResetTime(job.getResetKey(inbound))
	if !retrievedTime.Equal(testTime) {
		t.Errorf("Expected %v, got %v", testTime, retrievedTime)
	}

	// Test updating existing reset time
	newTime := time.Date(2024, 1, 16, 0, 5, 0, 0, time.UTC)
	job.updateLastResetTime(inbound, newTime)

	retrievedTime = job.getLastResetTime(job.getResetKey(inbound))
	if !retrievedTime.Equal(newTime) {
		t.Errorf("Expected %v, got %v", newTime, retrievedTime)
	}
}

// Test for concurrent access to lastResetTimes map
func TestConcurrentAccess(t *testing.T) {
	job := NewPeriodicTrafficResetJob()

	inbound := &model.Inbound{
		Id:                   1,
		Tag:                  "concurrent-test",
		PeriodicTrafficReset: "daily",
	}

	// Run multiple goroutines to test concurrent access
	done := make(chan bool, 20)

	// 10 writers
	for i := 0; i < 10; i++ {
		go func(i int) {
			testTime := time.Date(2024, 1, 15+i, 0, 5, 0, 0, time.UTC)
			job.updateLastResetTime(inbound, testTime)
			done <- true
		}(i)
	}

	// 10 readers
	for i := 0; i < 10; i++ {
		go func() {
			_ = job.getLastResetTime(job.getResetKey(inbound))
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 20; i++ {
		<-done
	}

	// If we reach here without panic/deadlock, the concurrent access is safe
	t.Log("Concurrent access test passed")
}

func TestValidResetTypes(t *testing.T) {
	expectedTypes := []string{"never", "daily", "weekly", "monthly"}
	
	for _, resetType := range expectedTypes {
		if !validResetTypes[resetType] {
			t.Errorf("Expected reset type %s to be valid", resetType)
		}
	}

	// Test invalid type
	if validResetTypes["invalid"] {
		t.Error("Expected 'invalid' to not be a valid reset type")
	}
}
