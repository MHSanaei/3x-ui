package job

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/util/sys"
)

// CheckConntrackJob reports pressure and recovery of the host conntrack table.
type CheckConntrackJob struct {
	mu        sync.Mutex
	readUsage func() (uint64, uint64, error)
	enabled   func() bool
	high      bool
}

func NewCheckConntrackJob(enabled func() bool) *CheckConntrackJob {
	return &CheckConntrackJob{readUsage: sys.GetConntrackUsage, enabled: enabled}
}

func (j *CheckConntrackJob) Run() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.enabled() {
		j.high = false
		return
	}

	count, max, err := j.readUsage()
	if err != nil || max == 0 || EventBus == nil {
		return
	}

	percent := float64(count) / float64(max) * 100
	typ := eventbus.EventConntrackHigh
	switch {
	case percent >= 80:
		// Repeats refresh the live notice and retry failed deliveries.
		j.high = true
	case j.high && percent < 70:
		j.high = false
		typ = eventbus.EventConntrackRecovered
	default:
		return
	}
	EventBus.Publish(eventbus.Event{
		Type: typ,
		Data: &eventbus.ConntrackData{Count: count, Max: max, Percent: percent},
	})
}
