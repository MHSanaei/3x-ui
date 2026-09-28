# Precise policy and accounting semantics

This is the target contract. See validation.md before treating any item as
implemented or verified.

- Directions are from the client's perspective. Raw counters count accepted
  application payload at exactly one authenticated dispatch point. Each
  backend must declare differences before it can participate in billing.
- Rates are nonnegative integer bytes/second. Zero means unlimited. UI Mbps
  uses 1,000,000 bits/s; MB/s uses 1,000,000 bytes/s. GiB uses 1,073,741,824
  bytes; GB uses 1,000,000,000. No multiplier enters the rate calculation.
- Multiplier is a positive decimal in [0.001, 1000], at most three fractional
  digits, stored as integer milli-units; default 1000 (=1×). Reject zero,
  negatives, NaN, Infinity, exponents and excess precision. API uses an exact
  decimal string; legacy omission defaults to 1× at the model boundary.
- Raw bytes and billed whole bytes use signed 64-bit nonnegative integers.
  Billed remainder is [0,999] thousandths of a byte. Overflow is an error,
  never wrapping, silent clamping or free traffic.
- For delta N and multiplier M, add N*M milli-bytes to the carried remainder,
  extract whole billed bytes, retain the remainder. Compute with checked
  quotient/remainder arithmetic so N*M need not fit int64. This makes batch
  splitting invariant. Persist all components in the same transaction.
- Multiplier revision applies only after a settled boundary. For 10 GiB at
  1× then 5 GiB at 2×, raw total=15 GiB and billed total=20 GiB. Historical
  use is never recomputed using the current multiplier.
- An unlimited quota is zero. Otherwise allowance is evaluated against billed
  bytes AND the fractional remainder. A 1-byte quota at 0.5× allows exactly
  2 raw bytes; at 1.5× it cannot forward one whole byte under strict admission.
- A client policy ID is independent of email/IP/port and is never recycled.
  Every counter source has a persisted incarnation and monotonic sequence.
  Counter reset requires a new accepted incarnation; a smaller count in the
  same incarnation is not inferred to be a restart.
- Report replay cannot change raw counters or charges. Out-of-order reports
  cannot rewind a cursor. All source contributions are settled once by their
  billing owner; transport bridges, outbound stats and master mirrors are not
  new chargeable sources.
- Manual-disabled, expired, quota-depleted and backend-unavailable are
  independent restriction reasons. Reset/renew/increase-quota clears only the
  relevant reason. Admission and existing flows enforce their union.
- Aggregate shaping scope is immutable client across every local connection,
  channel and binding. Global multi-node caps require allocated shares, not
  one full bucket per node. Policy updates affect live waiters within 2s.
- Before throughput acceptance, fix burst/queue limits and measurement window.
  Allowed upper error is burst/window plus measurement-clock error, not an
  arbitrary percent adjusted after failure. Verify a meaningful lower bound.
- Before quota acceptance, fix reservation size, concurrency and flush/lease
  durations. Derive crash loss, cutoff delay and maximum overuse from those
  limits; independently measure TCP, UDP and racing connections. These bounds
  are not yet established for the final adapters and remain open acceptance
  items, not production guarantees.
