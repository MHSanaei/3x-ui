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
		return fmt.Sprintf("GREATEST(%s, %s)", a, b)
	}
	return fmt.Sprintf("MAX(%s, %s)", a, b)
}
