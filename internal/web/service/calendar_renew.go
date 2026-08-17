package service

import "time"

// nextCalendarRenewal returns the next renewal instant strictly after from, on
// the given day of the month at midnight in loc.
//
// A day that does not exist in the target month falls back to that month's last
// day, so a client billed on the 31st renews on 28 February and returns to the
// 31st in March rather than drifting to the 3rd (#6071).
func nextCalendarRenewal(from time.Time, day int, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	if day < 1 {
		day = 1
	}
	if day > 31 {
		day = 31
	}
	local := from.In(loc)

	candidate := calendarDay(local.Year(), local.Month(), day, loc)
	if !candidate.After(local) {
		year, month := local.Year(), local.Month()+1
		if month > time.December {
			year, month = year+1, time.January
		}
		candidate = calendarDay(year, month, day, loc)
	}
	return candidate
}

// calendarDay builds midnight on the requested day, clamped to the last day the
// month actually has. time.Date would roll 31 February forward into March,
// which is the drift this whole mode exists to avoid.
func calendarDay(year int, month time.Month, day int, loc *time.Location) time.Time {
	last := daysInMonth(year, month)
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
