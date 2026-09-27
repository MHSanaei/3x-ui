package sub

import (
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func (s *SubService) subscriptionUserinfo(traffic xray.ClientTraffic) string {
	expire := traffic.ExpiryTime / 1000
	if s.subCalendarExpireInclusive && traffic.ResetDay == 1 && traffic.ExpiryTime > 0 && s.calendarExpireLocation != nil {
		at := time.UnixMilli(traffic.ExpiryTime).In(s.calendarExpireLocation)
		midnight := at.Day() == 1 && at.Hour() == 0 && at.Minute() == 0 && at.Second() == 0 && at.Nanosecond() == 0
		if midnight && at.Add(-time.Second).Month() != at.Month() {
			// Opt-in last-valid-second presentation; never change the real cutoff (#6516).
			expire--
		}
	}
	return fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, expire)
}
