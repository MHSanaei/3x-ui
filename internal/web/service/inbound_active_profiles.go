package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// GetClientActiveProfiles never infers client attribution from unrelated
// inbound traffic or source IPs shared by several users behind the same NAT.
func (s *InboundService) GetClientActiveProfiles() xray.ActiveProfileTree {
	out := xray.ActiveProfileTree{}
	process := currentXrayProcess()
	if process == nil {
		return out
	}
	now := time.Now()
	out = process.GetMergedActiveProfiles(now)
	if guid := s.panelGuid(); guid != "" {
		local := xray.ActiveProfileSnapshot{Clients: map[string][]string{}}
		if process.IsRunning() {
			local = process.GetLocalActiveProfiles(now)
		}
		out[guid] = local
	}
	return out
}
