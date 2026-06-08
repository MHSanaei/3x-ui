package database

import "fmt"

// JSONClientsFromInbound returns the FROM clause that yields one row per element
// of inbounds.settings -> clients, with a column named `client.value` whose text
// fields can be read with JSONFieldText("client.value", "<key>").
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

// ClientTrafficEnableMergeExpr returns the SQL expression used in the
// node traffic merge to update client_traffics.enable.
//
// The intent is: only allow the remote node to *disable* a client
// (never re-enable one that the central panel has disabled).
//
// We use a dialect-specific expression because:
// - On PostgreSQL we want strict boolean typing and casts to avoid
//   "CASE types boolean and integer cannot be matched" errors
//   (and similar internal expansions of AND/GREATEST).
// - On SQLite, enable is stored with INTEGER affinity (0/1), there is
//   no :: cast syntax, and we must produce a numeric-compatible result.
//
// The expression must be valid SQL for tx.Exec with a boolean parameter
// as the first ?.
func ClientTrafficEnableMergeExpr() string {
	if IsPostgres() {
		return "CASE WHEN ?::boolean THEN enable::boolean ELSE false END"
	}
	// SQLite: no :: casts. Use numeric CASE. 1/0 work as true/false
	// thanks to SQLite's affinity and how GORM/drivers bind bools.
	return "CASE WHEN ? THEN enable ELSE 0 END"
}
