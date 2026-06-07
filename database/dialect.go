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

// ClientTrafficEnableMergeExpr returns a dialect-safe SQL expression for
// the node traffic merge's client_traffics.enable update.
//
// Logic: only allow the remote node to force-disable (never re-enable
// a client the central has disabled). This prevents the node from
// "reviving" depleted clients during sync.
//
// PG: strict boolean with casts to avoid "CASE types boolean and integer"
//     errors from internal expansions of AND/GREATEST/CASE.
// SQLite: numeric 0/1 (matches column affinity and how GORM binds bools).
func ClientTrafficEnableMergeExpr() string {
	if IsPostgres() {
		return "CASE WHEN ?::boolean THEN enable::boolean ELSE false END"
	}
	return "CASE WHEN ? THEN enable ELSE 0 END"
}
