// Package traffic holds the node billed-traffic multiplier accounting.
//
// Multipliers are fixed-point hundredths: 100 = 1x, 150 = 1.5x, 200 = 2x.
// Only per-sync deltas are ever multiplied — historical totals, baselines
// and adoption seeds always stay raw, so re-syncs never double-bill and a
// multiplier change only affects traffic produced afterwards.
//
// The accepted range is [100, 10000]: sub-1x billing is intentionally not
// supported. A node's local (raw) traffic switch is decided by the raw
// counters, so a sub-1x node would report less billed traffic than the raw
// usage its local limits enforce — an operator-confusing mismatch. Values
// below the range are treated as corrupt and heal to 1x.
//
// Scope: the multiplier is a property of a node (nodes.traffic_multiplier) and
// is applied on the node-sync path only. The local master has no node row, so
// its own inbounds and clients keep billing 1:1 — there is deliberately no
// multiplier hook on the local accumulation path.
package traffic

const (
	// DefaultMultiplier bills traffic 1:1.
	DefaultMultiplier int64 = 100
	// MinMultiplier is the smallest accepted value (1x); sub-1x is rejected.
	MinMultiplier int64 = 100
	// MaxMultiplier is the largest accepted value (100x).
	MaxMultiplier int64 = 10000
)

// Normalize returns m when it is inside [MinMultiplier, MaxMultiplier],
// otherwise DefaultMultiplier, so a zero-valued or corrupt row can never
// silently zero out or explode billing.
func Normalize(m int64) int64 {
	if m < MinMultiplier || m > MaxMultiplier {
		return DefaultMultiplier
	}
	return m
}

// Validate reports whether m is an acceptable operator-supplied multiplier.
func Validate(m int64) bool {
	return m >= MinMultiplier && m <= MaxMultiplier
}

// Apply scales a raw per-sync delta by m/100 using integer arithmetic.
// Negative deltas (counter resets) bill as zero, matching the raw-clamp
// behavior of the sync loop. The product is floor-divided: sub-unit
// remainders are truncated within a tick, which is negligible because a
// sync tick moves far more than 100 bytes in practice, while baselines stay
// raw so no systematic error accumulates across ticks.
//
// At 1x the delta is returned untouched, so a default install (no node has
// ever been given a multiplier) stays on exactly the same integer path it
// ran before this package existed.
func Apply(delta, m int64) int64 {
	if delta <= 0 {
		return 0
	}
	m = Normalize(m)
	if m == DefaultMultiplier {
		return delta
	}
	return delta * m / 100
}
