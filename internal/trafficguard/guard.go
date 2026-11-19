// Package trafficguard evaluates timestamped per-user increments, never lifetime counters.
package trafficguard

import "fmt"

// Config uses decimal bytes (MB = 1e6, GB = 1e9) and milliseconds.
type Config struct {
	RateEnabled, WindowEnabled                                            bool
	RateBytesPerSecond, RateDurationMS, WindowBytes, WindowMS, CooldownMS int64
}

func (c Config) Revision() string { return fmt.Sprintf("%+v", c) }

// State is persisted by the caller. A process restart must call BreakContinuity.
type State struct {
	Revision                       string
	LastEnd, RateSince, RateBytes  int64
	RateNotified, WindowNotified   bool
	LastRateAlert, LastWindowAlert int64
}

type Detection struct {
	Rule                         string
	Bytes, Threshold, DurationMS int64
}

func (s *State) BreakContinuity() {
	s.LastEnd, s.RateSince, s.RateBytes = 0, 0, 0
	s.RateNotified = false
}

// Step consumes a valid interval and the complete-interval rolling sum.
// Missing intervals must break continuity; low/zero rates reset the streak.
func Step(s *State, c Config, start, end, used, windowBytes int64) []Detection {
	if end <= start || used < 0 {
		s.BreakContinuity()
		return nil
	}
	if s.Revision != c.Revision() {
		s.BreakContinuity()
		s.WindowNotified = false
		s.Revision = c.Revision()
	}
	if s.LastEnd != start {
		s.BreakContinuity()
	}
	var found []Detection
	if c.RateEnabled && float64(used)*1000/float64(end-start) > float64(c.RateBytesPerSecond) {
		if s.RateSince == 0 {
			s.RateSince = start
		}
		s.RateBytes += used
		if end-s.RateSince >= c.RateDurationMS && !s.RateNotified &&
			(s.LastRateAlert == 0 || end-s.LastRateAlert >= c.CooldownMS) {
			found = append(found, Detection{"rate", s.RateBytes, c.RateBytesPerSecond, end - s.RateSince})
			s.RateNotified, s.LastRateAlert = true, end
		}
	} else {
		s.BreakContinuity()
	}
	s.LastEnd = end
	if !c.WindowEnabled || windowBytes <= c.WindowBytes {
		s.WindowNotified = false
	} else if !s.WindowNotified && (s.LastWindowAlert == 0 || end-s.LastWindowAlert >= c.CooldownMS) {
		found = append(found, Detection{"window", windowBytes, c.WindowBytes, c.WindowMS})
		s.WindowNotified, s.LastWindowAlert = true, end
	}
	return found
}
