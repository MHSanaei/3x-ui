package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/iptables"
)

const maxBlockAgeSecs int64 = 600 // 10 minutes

// UnblockIPsJob removes expired iptables DROP rules from the 3X-UI-BLOCK chain.
// Rules older than maxBlockAgeSecs are removed to prevent the firewall table
// from growing unbounded and to unblock IPs that may have been re-assigned.
type UnblockIPsJob struct{}

// NewUnblockIPsJob creates a new instance of the IP unblock cleanup job.
func NewUnblockIPsJob() *UnblockIPsJob {
	return &UnblockIPsJob{}
}

// Run enumerates all rules in the 3X-UI-BLOCK chain and removes any that are
// older than maxBlockAgeSecs.
func (j *UnblockIPsJob) Run() {
	rules, err := iptables.ListRules()
	if err != nil {
		logger.Debug("UnblockIPsJob: failed to list iptables rules:", err)
		return
	}
	now := time.Now().Unix()
	for _, rule := range rules {
		if rule.InsertedAt > 0 && (now-rule.InsertedAt) > maxBlockAgeSecs {
			if err := iptables.UnblockIP(rule.IP, rule.Port); err != nil {
				logger.Warning("UnblockIPsJob: failed to unblock", rule.IP, rule.Port, err)
			} else {
				logger.Debug("UnblockIPsJob: unblocked expired rule", rule.IP, rule.Port)
			}
		}
	}
}
