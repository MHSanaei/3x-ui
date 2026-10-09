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

// SessionObservation is one report of client activity, folded into the session
// columns of its client_traffics row by ClientSessionAssignments.
type SessionObservation struct {
	QuietBefore      int64 // a last_online older than this means the previous session ended
	UntrackedBefore  int64 // a row with no session yet opens one only below this last_online
	Start            int64 // session_start of a session this report opens
	OpenUp, OpenDown int64 // counters a session this report opens starts from
	Up, Down         int64 // bytes the running session gains otherwise
}

// ClientSessionAssignments returns the SET list folding o into the session
// columns, with its args in placeholder order; it reads last_online's old value.
func ClientSessionAssignments(o SessionObservation) (string, []any) {
	opens := "(last_online < CAST(? AS BIGINT) OR (session_start <= 0 AND last_online < CAST(? AS BIGINT)))"
	set := fmt.Sprintf(
		"session_start = CASE WHEN %[1]s THEN CAST(? AS BIGINT) ELSE session_start END, "+
			"session_up = CASE WHEN %[1]s THEN CAST(? AS BIGINT) ELSE %[2]s END, "+
			"session_down = CASE WHEN %[1]s THEN CAST(? AS BIGINT) ELSE %[3]s END",
		opens, ClampedAddExpr("session_up"), ClampedAddExpr("session_down"),
	)
	return set, []any{
		o.QuietBefore, o.UntrackedBefore, o.Start,
		o.QuietBefore, o.UntrackedBefore, o.OpenUp, o.Up,
		o.QuietBefore, o.UntrackedBefore, o.OpenDown, o.Down,
	}
}

// ClientTrafficEnableMergeExpr: placeholders nodeEnable, nodeExpiry, nodeTotal,
// now, deltaUp, deltaDown. Mirrors nodeDisableIsStale (#6228 / #4917).
func ClientTrafficEnableMergeExpr() string {
	if IsPostgres() {
		return `CASE
			WHEN ?::boolean THEN enable::boolean
			WHEN (expiry_time <> CAST(? AS BIGINT) OR total <> CAST(? AS BIGINT))
				AND (expiry_time <= 0 OR expiry_time > CAST(? AS BIGINT))
				AND (total <= 0 OR up + ? + down + ? < total) THEN enable::boolean
			ELSE false
		END`
	}
	return `CASE
		WHEN ? THEN enable
		WHEN (expiry_time <> CAST(? AS BIGINT) OR total <> CAST(? AS BIGINT))
			AND (expiry_time <= 0 OR expiry_time > CAST(? AS BIGINT))
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
