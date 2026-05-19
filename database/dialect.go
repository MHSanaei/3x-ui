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

// JSONFieldText returns a SQL expression that extracts the textual value of <key>
// from a JSON expression. On both backends the result is the raw (unquoted) string,
// so callers do NOT need to trim surrounding quotes.
func JSONFieldText(expr, key string) string {
	if IsPostgres() {
		return fmt.Sprintf("(%s ->> '%s')", expr, key)
	}
	// SQLite's JSON_EXTRACT on a text value returns the JSON-encoded form
	// (with surrounding quotes). Wrap it in json_extract(json_quote(...)) trick
	// is fragile; simpler: unwrap quotes with TRIM(BOTH '"').
	return fmt.Sprintf("TRIM(JSON_EXTRACT(%s, '$.%s'), '\"')", expr, key)
}
