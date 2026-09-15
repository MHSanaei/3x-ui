package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

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

// Advance by calendar dates, not 168 hours; a repeated midnight must not spend
// another weekly allowance on the same local date.
func nextWeeklyRenewal(from time.Time, weekday int, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	local := from.In(loc)
	days := (min(7, max(1, weekday))%7 - int(local.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	date := time.Date(local.Year(), local.Month(), local.Day()+days, 0, 0, 0, 0, time.UTC)
	// A corrupt or unusual zone must not stall the single traffic writer.
	// Returning from lets the catch-up forward-progress guard fail closed.
	for range 8 {
		candidate, exists := localCalendarDateStart(date, loc)
		if exists && candidate.After(local) {
			return candidate
		}
		date = date.AddDate(0, 0, 7)
	}
	return from
}

// Find the first valid instant of a local date: Date can pick a repeated
// midnight or normalize a nonexistent midnight into the preceding day.
func localCalendarDateStart(date time.Time, loc *time.Location) (time.Time, bool) {
	localDate := func(at time.Time) time.Time {
		local := at.In(loc)
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	}
	candidate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	if localDate(candidate).Equal(date) && localDate(candidate.Add(-time.Second)).Before(date) {
		return candidate, true
	}
	low, high := candidate.Unix()-86400, candidate.Unix()+86400
	for low < high {
		mid := low + (high-low)/2
		if localDate(time.Unix(mid, 0)).Before(date) {
			low = mid + 1
		} else {
			high = mid
		}
	}
	start := time.Unix(low, 0).In(loc)
	return start, localDate(start).Equal(date)
}

func nextClientRenewal(expiry int64, reset, day, weekday int, loc *time.Location) int64 {
	if day > 0 {
		return nextCalendarRenewal(time.UnixMilli(expiry), day, loc).UnixMilli()
	}
	if weekday > 0 {
		return nextWeeklyRenewal(time.UnixMilli(expiry), weekday, loc).UnixMilli()
	}
	return expiry + int64(reset)*86400000
}

func canonicalRenewalExpiry(expiry int64, reset, day, weekday int, loc *time.Location) int64 {
	if day > 0 || weekday > 0 {
		boundary := nextClientRenewal(expiry, reset, day, weekday, loc)
		if expiry >= boundary-1000 && expiry < boundary {
			return boundary
		}
	}
	return expiry
}

func catchUpClientRenewal(traffic *xray.ClientTraffic, now int64, loc *time.Location) (int64, int) {
	if traffic.ResetDay <= 0 && traffic.ResetWeekday <= 0 && traffic.Reset <= 0 {
		return traffic.ExpiryTime, 0
	}
	expiry := canonicalRenewalExpiry(traffic.ExpiryTime, traffic.Reset, traffic.ResetDay, traffic.ResetWeekday, loc)
	renewals := 0
	for expiry < now {
		if traffic.ResetMax > 0 && traffic.ResetCount+renewals >= traffic.ResetMax {
			break
		}
		next := nextClientRenewal(expiry, traffic.Reset, traffic.ResetDay, traffic.ResetWeekday, loc)
		if next <= expiry {
			return traffic.ExpiryTime, 0
		}
		expiry = next
		renewals++
	}
	if renewals == 0 {
		return traffic.ExpiryTime, 0
	}
	return expiry, renewals
}
