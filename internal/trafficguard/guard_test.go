package trafficguard

import "testing"

func config() Config {
	return Config{RateEnabled: true, WindowEnabled: true, RateBytesPerSecond: 7_000_000, RateDurationMS: 60_000, WindowBytes: 5_000_000_000, WindowMS: 1_800_000, CooldownMS: 3_600_000}
}

func TestSustainedNeedsEveryIntervalAndElapsedMinute(t *testing.T) {
	var s State
	c := config()
	start := int64(100000)
	for i := int64(0); i < 12; i++ {
		found := Step(&s, c, start+i*5000, start+(i+1)*5000, 40_000_000, 0)
		if (len(found) == 1) != (i == 11) {
			t.Fatalf("interval %d: %v", i, found)
		}
	}
	if len(Step(&s, c, start+60000, start+65000, 40_000_000, 0)) != 0 {
		t.Fatal("duplicate in same episode")
	}
	// A zero interval and a single burst cannot satisfy the sustained rule.
	Step(&s, c, start+65000, start+70000, 0, 0)
	if len(Step(&s, c, start+70000, start+75000, 500_000_000, 0)) != 0 {
		t.Fatal("burst counted as sustained")
	}
}

func TestExactRateThresholdAndGapReset(t *testing.T) {
	c := config()
	var s State
	for i := int64(0); i < 20; i++ {
		if len(Step(&s, c, 100000+i*5000, 105000+i*5000, 35_000_000, 0)) != 0 {
			t.Fatal("equal threshold fired")
		}
	}
	for i := int64(0); i < 10; i++ {
		Step(&s, c, 200000+i*5000, 205000+i*5000, 40_000_000, 0)
	}
	if len(Step(&s, c, 260000, 265000, 40_000_000, 0)) != 0 || s.RateSince != 260000 {
		t.Fatal("missing interval was bridged")
	}
}

func TestRollingThresholdCooldownRearmAndRestart(t *testing.T) {
	c := config()
	var s State
	if len(Step(&s, c, 100000, 105000, 0, c.WindowBytes)) != 0 {
		t.Fatal("equal window threshold fired")
	}
	a := Step(&s, c, 105000, 110000, 1, c.WindowBytes+1)
	if len(a) != 1 || a[0].Rule != "window" {
		t.Fatalf("missing window alert: %v", a)
	}
	s.BreakContinuity() // restoring persisted cooldown must suppress a duplicate
	if len(Step(&s, c, 120000, 125000, 0, c.WindowBytes+1)) != 0 {
		t.Fatal("restart duplicated window episode")
	}
	Step(&s, c, 125000, 130000, 0, 0)
	if len(Step(&s, c, 130000, 135000, 1, c.WindowBytes+1)) != 0 {
		t.Fatal("cooldown not applied")
	}
	if len(Step(&s, c, 4000000, 4005000, 1, c.WindowBytes+1)) != 1 {
		t.Fatal("new episode did not rearm after cooldown")
	}
}

func TestConfigurationChangeAndInvalidIntervals(t *testing.T) {
	c := config()
	var s State
	for i := int64(0); i < 11; i++ {
		Step(&s, c, 100000+i*5000, 105000+i*5000, 40_000_000, 0)
	}
	c.RateBytesPerSecond = 6_000_000
	if len(Step(&s, c, 155000, 160000, 40_000_000, 0)) != 0 || s.RateSince != 155000 {
		t.Fatal("configuration change retained old streak")
	}
	Step(&s, c, 160000, 165000, -1, 0)
	if s.RateSince != 0 {
		t.Fatal("negative increment retained streak")
	}
	Step(&s, c, 165000, 165000, 1, 0)
	if s.LastEnd != 0 {
		t.Fatal("zero duration accepted")
	}
}
