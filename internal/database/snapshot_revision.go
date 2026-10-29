package database

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Database triggers cover direct SQL and background lifecycle edits, not just service writes.
func migrateSnapshotRevisions(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, entry := range []struct {
			value  any
			table  string
			ignore map[string]bool
		}{
			{&model.Inbound{}, "inbounds", map[string]bool{"up": true, "down": true}},
			{&model.ClientRecord{}, "clients", map[string]bool{"updated_at": true}},
		} {
			stmt := &gorm.Statement{DB: tx}
			if err := stmt.Parse(entry.value); err != nil {
				return err
			}
			var columns, comparisons []string
			for _, col := range stmt.Schema.DBNames {
				if col == "snapshot_revision" || entry.ignore[col] {
					continue
				}
				quoted := `"` + col + `"`
				columns = append(columns, quoted)
				if tx.Name() == DialectPostgres {
					comparisons = append(comparisons, "NEW."+quoted+" IS DISTINCT FROM OLD."+quoted)
				} else {
					comparisons = append(comparisons, "NEW."+quoted+" IS NOT OLD."+quoted)
				}
			}
			table := entry.table
			name := "snapshot_revision_" + table
			var commands []string
			if tx.Name() == DialectPostgres {
				commands = []string{
					fmt.Sprintf(`CREATE OR REPLACE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF TG_OP = 'INSERT' THEN NEW.snapshot_revision := md5(random()::text || clock_timestamp()::text); ELSIF %s THEN NEW.snapshot_revision := md5(random()::text || clock_timestamp()::text); ELSE NEW.snapshot_revision := OLD.snapshot_revision; END IF; RETURN NEW; END $$`, name, strings.Join(comparisons, " OR ")),
					fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON %s`, name, table),
					fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT OR UPDATE OF %s ON %s FOR EACH ROW EXECUTE FUNCTION %s()`, name, strings.Join(columns, ","), table, name),
				}
			} else {
				commands = []string{
					fmt.Sprintf(`DROP TRIGGER IF EXISTS %s_insert`, name), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s_update`, name),
					fmt.Sprintf(`CREATE TRIGGER %s_insert AFTER INSERT ON %s BEGIN UPDATE %s SET snapshot_revision = lower(hex(randomblob(16))) WHERE id = NEW.id; END`, name, table, table),
					fmt.Sprintf(`CREATE TRIGGER %s_update AFTER UPDATE OF %s ON %s WHEN %s BEGIN UPDATE %s SET snapshot_revision = lower(hex(randomblob(16))) WHERE id = NEW.id; END`, name, strings.Join(columns, ","), table, strings.Join(comparisons, " OR "), table),
				}
			}
			tokenSQL := "lower(hex(randomblob(16)))"
			if tx.Name() == DialectPostgres {
				tokenSQL = "md5(random()::text || clock_timestamp()::text)"
			}
			commands = append(commands, fmt.Sprintf("UPDATE %s SET snapshot_revision = %s WHERE snapshot_revision = ''", table, tokenSQL))

			for _, sql := range commands {
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
