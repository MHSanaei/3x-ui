// Package cronspec parses the cron schedules an admin types into the settings.
package cronspec

import (
	"strings"

	"github.com/robfig/cron/v3"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// The panel scheduler runs WithSeconds(), whose parser rejects five-field crontab,
// so admin schedules are parsed here with the seconds column optional.
var parser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

func Parse(spec string) (cron.Schedule, error) {
	schedule, err := parser.Parse(strings.TrimSpace(spec))
	if err != nil {
		return nil, common.NewErrorf("cron schedule <%v> is not valid: %v", spec, err)
	}
	return schedule, nil
}
