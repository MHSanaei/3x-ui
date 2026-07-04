package database

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

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

func CastBigint(placeholder string) string {
	if IsPostgres() {
		return fmt.Sprintf("CAST(%s AS BIGINT)", placeholder)
	}
	return placeholder
}

type TrafficDelta struct {
	Tag  string
	Up   int64
	Down int64
}

type ClientTrafficDelta struct {
	Email      string
	Up         int64
	Down       int64
	LastOnline int64
}

const sqliteMaxVars = 999

func BatchIncrementInboundTraffic(tx *gorm.DB, deltas []TrafficDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	if IsPostgres() {
		return batchIncrementInboundTrafficPG(tx, deltas)
	}
	return batchIncrementInboundTrafficSQLite(tx, deltas)
}

func batchIncrementInboundTrafficPG(tx *gorm.DB, deltas []TrafficDelta) error {
	var sb strings.Builder
	args := make([]any, 0, len(deltas)*3)
	sb.WriteString("UPDATE inbounds SET up = inbounds.up + v.delta_up, down = inbounds.down + v.delta_down FROM (VALUES ")
	for i, d := range deltas {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?::text, ?::bigint, ?::bigint)")
		args = append(args, d.Tag, d.Up, d.Down)
	}
	sb.WriteString(") AS v(tag, delta_up, delta_down) WHERE inbounds.tag = v.tag AND inbounds.node_id IS NULL")
	return tx.Exec(sb.String(), args...).Error
}

// ponytail: 5 vars per row (2*CASE + IN), chunks at ~199 rows (~999 vars). Upgrade to temp-table
// approach if inbound count exceeds this.
func batchIncrementInboundTrafficSQLite(tx *gorm.DB, deltas []TrafficDelta) error {
	const varsPerRow = 5
	chunkSize := sqliteMaxVars / varsPerRow
	for start := 0; start < len(deltas); start += chunkSize {
		end := start + chunkSize
		if end > len(deltas) {
			end = len(deltas)
		}
		chunk := deltas[start:end]

		var sb strings.Builder
		args := make([]any, 0, len(chunk)*varsPerRow+len(chunk))
		sb.WriteString("UPDATE inbounds SET up = up + CASE tag")
		for _, d := range chunk {
			sb.WriteString(" WHEN ? THEN ?")
			args = append(args, d.Tag, d.Up)
		}
		sb.WriteString(" ELSE 0 END, down = down + CASE tag")
		for _, d := range chunk {
			sb.WriteString(" WHEN ? THEN ?")
			args = append(args, d.Tag, d.Down)
		}
		sb.WriteString(" ELSE 0 END WHERE node_id IS NULL AND tag IN (")
		for i, d := range chunk {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("?")
			args = append(args, d.Tag)
		}
		sb.WriteString(")")

		if err := tx.Exec(sb.String(), args...).Error; err != nil {
			return err
		}
	}
	return nil
}

func BatchIncrementClientTraffic(tx *gorm.DB, deltas []ClientTrafficDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	if IsPostgres() {
		return batchIncrementClientTrafficPG(tx, deltas)
	}
	return batchIncrementClientTrafficSQLite(tx, deltas)
}

func batchIncrementClientTrafficPG(tx *gorm.DB, deltas []ClientTrafficDelta) error {
	var sb strings.Builder
	args := make([]any, 0, len(deltas)*4)
	sb.WriteString("UPDATE client_traffics SET up = client_traffics.up + v.delta_up, down = client_traffics.down + v.delta_down, last_online = GREATEST(client_traffics.last_online, v.last_online) FROM (VALUES ")
	for i, d := range deltas {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?::text, ?::bigint, ?::bigint, ?::bigint)")
		args = append(args, d.Email, d.Up, d.Down, d.LastOnline)
	}
	sb.WriteString(") AS v(email, delta_up, delta_down, last_online) WHERE client_traffics.email = v.email")
	return tx.Exec(sb.String(), args...).Error
}

// ponytail: 4 vars per row + 1 for IN clause = 5 effective per row; chunks at
// ~199 rows. Upgrade to temp-table if client count per tick exceeds this.
func batchIncrementClientTrafficSQLite(tx *gorm.DB, deltas []ClientTrafficDelta) error {
	const varsPerRow = 5
	chunkSize := sqliteMaxVars / varsPerRow
	for start := 0; start < len(deltas); start += chunkSize {
		end := start + chunkSize
		if end > len(deltas) {
			end = len(deltas)
		}
		chunk := deltas[start:end]

		var sb strings.Builder
		args := make([]any, 0, len(chunk)*varsPerRow+len(chunk))
		sb.WriteString("UPDATE client_traffics SET up = up + CASE email")
		for _, d := range chunk {
			sb.WriteString(" WHEN ? THEN ?")
			args = append(args, d.Email, d.Up)
		}
		sb.WriteString(" ELSE 0 END, down = down + CASE email")
		for _, d := range chunk {
			sb.WriteString(" WHEN ? THEN ?")
			args = append(args, d.Email, d.Down)
		}
		sb.WriteString(" ELSE 0 END, last_online = CASE email")
		for _, d := range chunk {
			fmt.Fprintf(&sb, " WHEN ? THEN %s", GreatestExpr("last_online", "?"))
			args = append(args, d.Email, d.LastOnline)
		}
		sb.WriteString(" ELSE last_online END WHERE email IN (")
		for i, d := range chunk {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("?")
			args = append(args, d.Email)
		}
		sb.WriteString(")")

		if err := tx.Exec(sb.String(), args...).Error; err != nil {
			return err
		}
	}
	return nil
}
