package job

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/util/sys"
)

// CheckConntrackJob reports host conntrack usage for notification thresholds.
type CheckConntrackJob struct {
	mu        sync.Mutex
	readUsage func() (uint64, uint64, error)
	enabled   func() bool
}

func NewCheckConntrackJob(enabled func() bool) *CheckConntrackJob {
	return &CheckConntrackJob{readUsage: sys.GetConntrackUsage, enabled: enabled}
}

func (j *CheckConntrackJob) Run() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.enabled() {
		return
	}

	count, max, err := j.readUsage()
	if err != nil || max == 0 || EventBus == nil {
		return
	}

	percent := float64(count) / float64(max) * 100
	EventBus.Publish(eventbus.Event{
		Type: eventbus.EventConntrackHigh,
		Data: &eventbus.ConntrackData{Count: count, Max: max, Percent: percent},
	})
}
