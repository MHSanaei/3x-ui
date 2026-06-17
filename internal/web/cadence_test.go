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
		"cadenceXrayRunning": cadenceXrayRunning,
		"cadenceMtproto":     cadenceMtproto,
		"cadenceOutboundSub": cadenceOutboundSub,
		"cadenceCheckHash":   cadenceCheckHash,
		"cadenceCPUAlarm":    cadenceCPUAlarm,
	}
	for name, spec := range cadences {
		if _, err := cron.ParseStandard(spec); err != nil {
			t.Errorf("%s = %q is not a valid cron spec: %v", name, spec, err)
		}
	}
}

// everySeconds renders the operator-tunable interval settings into cron specs.
// It must emit a valid spec for any positive interval and fall back to the
// provided default when the getter errors or yields a non-positive value.
func TestEverySecondsProducesValidSpecs(t *testing.T) {
	cases := []struct {
		name string
		val  int
		err  bool
		def  int
		want string
	}{
		{"normal", 5, false, 5, "@every 5s"},
		{"large", 3600, false, 5, "@every 3600s"},
		{"zero falls back", 0, false, 7, "@every 7s"},
		{"negative falls back", -3, false, 7, "@every 7s"},
		{"error falls back", 0, true, 9, "@every 9s"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			getter := func() (int, error) {
				if c.err {
					return c.val, assertErr
				}
				return c.val, nil
			}
			got := everySeconds(getter, c.def)
			if got != c.want {
				t.Fatalf("everySeconds = %q, want %q", got, c.want)
			}
			if _, err := cron.ParseStandard(got); err != nil {
				t.Fatalf("%q is not a valid cron spec: %v", got, err)
			}
		})
	}
}

var assertErr = errTest("boom")

type errTest string

func (e errTest) Error() string { return string(e) }
