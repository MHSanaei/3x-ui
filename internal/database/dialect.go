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

// ClientTrafficEnableMergeExpr: placeholders nodeEnable, nodeExpiry, nodeExpiry,
// deltaUp, deltaDown. Quota check includes this statement's deltas (#6228/#4917).
func ClientTrafficEnableMergeExpr() string {
	if IsPostgres() {
		return `CASE
			WHEN ?::boolean THEN enable::boolean
			WHEN CAST(? AS BIGINT) > 0 AND expiry_time > CAST(? AS BIGINT)
				AND (total <= 0 OR up + ? + down + ? < total) THEN enable::boolean
			ELSE false
		END`
	}
	return `CASE
		WHEN ? THEN enable
		WHEN CAST(? AS BIGINT) > 0 AND expiry_time > CAST(? AS BIGINT)
			AND (total <= 0 OR up + ? + down + ? < total) THEN enable
		ELSE 0
	END`
}

// ClientTrafficExpiryMergeExpr: placeholder nodeExpiry once. Master absolute is
// kept; CAST avoids Postgres int4 inference on ms timestamps.
func ClientTrafficExpiryMergeExpr() string {
	return `CASE
		WHEN expiry_time > 0 THEN expiry_time
		ELSE CAST(? AS BIGINT)
	END`
}
