package database

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DumpSQLite writes a portable SQL text dump of the SQLite database at srcPath
// to outPath. The output mirrors the `sqlite3 .dump` format (schema + data +
// indexes wrapped in a transaction), so it can be rebuilt with RestoreSQLite or
// loaded by the sqlite3 CLI. The source database is opened read-only in effect
// and left untouched.
func DumpSQLite(srcPath, outPath string) error {
	data, err := DumpSQLiteToBytes(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

// DumpSQLiteToBytes builds the same `sqlite3 .dump`-style SQL text as DumpSQLite
// but returns it in memory, which the panel uses to stream a migration download.
func DumpSQLiteToBytes(srcPath string) ([]byte, error) {
	if _, err := os.Stat(srcPath); err != nil {
		return nil, fmt.Errorf("source sqlite not found at %s: %w", srcPath, err)
	}

	gdb, err := gorm.Open(sqlite.Open(srcPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	var b strings.Builder
	b.WriteString("PRAGMA foreign_keys=OFF;\n")
	b.WriteString("BEGIN TRANSACTION;\n")

	// Tables in creation order, each followed by its data.
	type object struct{ name, ddl string }
	var tables []object
	rows, err := sqlDB.Query(`SELECT name, sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND sql IS NOT NULL ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var o object
		if err := rows.Scan(&o.name, &o.ddl); err != nil {
			rows.Close()
			return nil, err
		}
		tables = append(tables, o)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for _, t := range tables {
		b.WriteString(t.ddl)
		b.WriteString(";\n")
		if err := dumpTableData(sqlDB, t.name, &b); err != nil {
			return nil, err
		}
	}

	// AUTOINCREMENT bookkeeping, restored verbatim like the sqlite3 CLI does.
	if sqliteTableExists(sqlDB, "sqlite_sequence") {
		b.WriteString("DELETE FROM sqlite_sequence;\n")
		if err := dumpTableData(sqlDB, "sqlite_sequence", &b); err != nil {
			return nil, err
		}
	}

	// Indexes, triggers and views after the data is in place.
	rows2, err := sqlDB.Query(`SELECT sql FROM sqlite_master WHERE type IN ('index','trigger','view') AND sql IS NOT NULL ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	for rows2.Next() {
		var ddl string
		if err := rows2.Scan(&ddl); err != nil {
			rows2.Close()
			return nil, err
		}
		b.WriteString(ddl)
		b.WriteString(";\n")
	}
	if err := rows2.Err(); err != nil {
		rows2.Close()
		return nil, err
	}
	rows2.Close()

	b.WriteString("COMMIT;\n")

	return []byte(b.String()), nil
}

// RestoreSQLite rebuilds a SQLite database at dstPath from a SQL text dump
// produced by DumpSQLite (or `sqlite3 .dump`). dstPath must not already exist so
// an existing database is never clobbered silently.
func RestoreSQLite(dumpPath, dstPath string) error {
	script, err := os.ReadFile(dumpPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dstPath); err == nil {
		return fmt.Errorf("destination already exists: %s", dstPath)
	}

	gdb, err := gorm.Open(sqlite.Open(dstPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}

	// mattn/go-sqlite3 executes every statement in a multi-statement string.
	if _, err := sqlDB.Exec(string(script)); err != nil {
		sqlDB.Close()
		os.Remove(dstPath)
		return fmt.Errorf("restore failed: %w", err)
	}
	return sqlDB.Close()
}

// dumpTableData appends one INSERT statement per row of table to b.
func dumpTableData(db *sql.DB, table string, b *strings.Builder) error {
	rows, err := db.Query(`SELECT * FROM "` + table + `"`)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	n := len(cols)
	prefix := `INSERT INTO "` + table + `" VALUES(`

	for rows.Next() {
		vals := make([]any, n)
		ptrs := make([]any, n)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		b.WriteString(prefix)
		for i, v := range vals {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(sqliteLiteral(v))
		}
		b.WriteString(");\n")
	}
	return rows.Err()
}

// sqliteLiteral renders a scanned column value as a SQLite SQL literal.
func sqliteLiteral(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case bool:
		if x {
			return "1"
		}
		return "0"
	case string:
		return quoteSQLiteText(x)
	case []byte:
		if utf8.Valid(x) {
			return quoteSQLiteText(string(x))
		}
		var sb strings.Builder
		sb.WriteString("X'")
		for _, c := range x {
			fmt.Fprintf(&sb, "%02x", c)
		}
		sb.WriteByte('\'')
		return sb.String()
	default:
		return quoteSQLiteText(fmt.Sprintf("%v", x))
	}
}

func quoteSQLiteText(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func sqliteTableExists(db *sql.DB, name string) bool {
	var found string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&found)
	return err == nil
}
