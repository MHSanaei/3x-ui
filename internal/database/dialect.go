package database

import "fmt"

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

func ClientTrafficEnableMergeExpr() string {
	if IsPostgres() {
		return "CASE WHEN ?::boolean THEN enable::boolean ELSE false END"
	}
	return "CASE WHEN ? THEN enable ELSE 0 END"
}

// ClientTrafficExpiryMergeExpr returns the SQL expression for merging expiry_time.
// "Start after first connect" persists a negative duration that each node converts
// to an absolute deadline (now+duration) the first time the client connects there.
// The per-email client_traffics row is shared across every node, so a node that
// has not yet seen a first connection keeps reporting the negative duration —
// which must never reset a deadline another node already activated.
func ClientTrafficExpiryMergeExpr() string {
	if IsPostgres() {
		return "CASE WHEN ? > 0 AND ? <= 0 THEN ? ELSE ? END"
	}
	return "CASE WHEN ? > 0 AND ? <= 0 THEN ? ELSE ? END"
}

