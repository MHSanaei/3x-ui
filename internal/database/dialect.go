package database

import "fmt"

// TrafficMax caps every traffic counter safely below math.MaxInt64 (~9.22e18)
// so that one more delta can never overflow int64. SQLite silently promotes an
// overflowing INTEGER to REAL, after which the column no longer scans into the
// Go int64 field and every reader of the table fails (#5762).
const TrafficMax = int64(9_000_000_000_000_000_000)

func ClampedAddExpr(col string) string {
	if IsPostgres() {
		return fmt.Sprintf("LEAST(%s + ?, %d)", col, TrafficMax)
	}
	return fmt.Sprintf("MIN(%s + ?, %d)", col, TrafficMax)
}

func JSONClientsFromInbound() string {
	if IsPostgres() {
		return "FROM inbounds, jsonb_array_elements(inbounds.settings::jsonb -> 'clients') AS client(value)"
	}
	return "FROM inbounds, JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client"
}

func JSONFieldText(expr, key string) string {
	if IsPostgres() {
		return fmt.Sprintf("(%s ->> '%s')", expr, key)
	}

	return fmt.Sprintf("TRIM(JSON_EXTRACT(%s, '$.%s'), '\"')", expr, key)
}

func GreatestExpr(a, b string) string {
	if IsPostgres() {
		return fmt.Sprintf("GREATEST(%s::bigint, %s::bigint)", a, b)
	}
	return fmt.Sprintf("MAX(%s, %s)", a, b)
}

// ClientTrafficEnableMergeExpr merges a node-reported enable into
// client_traffics.enable. Placeholders (in order): nodeEnable, nodeExpiry,
// nodeExpiry. Mirrors staleNodeDisable / #4917 / #6228.
//
// A disable with an older absolute expiry is ignored only when the master row
// is not already over quota (total <= 0 OR up+down < total), so a genuine
// quota latch still wins even if the node's expiry lags the merged max.
func ClientTrafficEnableMergeExpr() string {
	if IsPostgres() {
		return `CASE
			WHEN ?::boolean THEN enable::boolean
			WHEN CAST(? AS BIGINT) > 0 AND expiry_time > CAST(? AS BIGINT)
				AND (total <= 0 OR up + down < total) THEN enable::boolean
			ELSE false
		END`
	}
	return `CASE
		WHEN ? THEN enable
		WHEN CAST(? AS BIGINT) > 0 AND expiry_time > CAST(? AS BIGINT)
			AND (total <= 0 OR up + down < total) THEN enable
		ELSE 0
	END`
}

// ClientTrafficExpiryMergeExpr merges a node-reported expiry into
// client_traffics.expiry_time. Placeholders (in order): nodeExpiry, nodeExpiry,
// nodeExpiry. Mirrors mergeActivationExpiry.
//
// CAST(? AS BIGINT) is required on Postgres: without it the `<= 0` / `<`
// comparisons infer int4 from the literal and overflow on real millisecond
// timestamps. The casts are kept on SQLite too so both dialects share one
// expression.
func ClientTrafficExpiryMergeExpr() string {
	return `CASE
		WHEN expiry_time > 0 AND (CAST(? AS BIGINT) <= 0 OR CAST(? AS BIGINT) < expiry_time) THEN expiry_time
		ELSE CAST(? AS BIGINT)
	END`
}
