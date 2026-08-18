package service

import "time"

// nextCalendarRenewal returns the next renewal strictly after from, at midnight
// in loc; a missing day clamps to the month's last, so the 31st comes back (#6106).
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

// Clamped rather than normalized: time.Date rolls 31 February into March, which
// is the drift this mode exists to avoid.
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
