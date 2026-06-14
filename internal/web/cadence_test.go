package web

import (
	"testing"

	"github.com/robfig/cron/v3"
)

// All centralized background-job cadences must remain valid cron specs. This is
// the guard for the "single tuning surface" refactor: editing a cadence to an
// invalid spec fails here instead of silently dropping a job at startup.
//
// NOTE: package web embeds the built frontend (//go:embed all:dist), so this
// test compiles only after `npm run build` has populated web/dist — the normal
// repo build flow.
func TestJobCadencesAreValidCronSpecs(t *testing.T) {
	cadences := map[string]string{
		"cadenceXrayRunning":   cadenceXrayRunning,
		"cadenceXrayRestart":   cadenceXrayRestart,
		"cadenceXrayTraffic":   cadenceXrayTraffic,
		"cadenceMtproto":       cadenceMtproto,
		"cadenceClientIPScan":  cadenceClientIPScan,
		"cadenceNodeHeartbeat": cadenceNodeHeartbeat,
		"cadenceNodeTraffic":   cadenceNodeTraffic,
		"cadenceOutboundSub":   cadenceOutboundSub,
		"cadenceCheckHash":     cadenceCheckHash,
		"cadenceCPUAlarm":      cadenceCPUAlarm,
	}
	for name, spec := range cadences {
		if _, err := cron.ParseStandard(spec); err != nil {
			t.Errorf("%s = %q is not a valid cron spec: %v", name, spec, err)
		}
	}
}
