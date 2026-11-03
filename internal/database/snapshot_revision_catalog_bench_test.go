package database

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func BenchmarkSnapshotRevisionCatalogCounters(b *testing.B) {
	clients := make([]model.Client, 2500)
	for i := range clients {
		clients[i] = model.Client{Email: fmt.Sprintf("client-%d", i), ID: fmt.Sprintf("uuid-%d", i), Enable: true, TotalGB: 1 << 30}
	}
	settings, err := json.Marshal(map[string]any{"clients": clients})
	if err != nil {
		b.Fatal(err)
	}
	for _, dialect := range []string{DialectSQLite, DialectPostgres} {
		b.Run(dialect, func(b *testing.B) {
			for _, mode := range []string{"scoped-baseline", "generic"} {
				b.Run(mode, func(b *testing.B) {
					db, _ := snapshotRevisionDDLDB(b, dialect)
					if mode == "scoped-baseline" {
						var definition string
						query := "SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = 'snapshot_revision_inbounds_update'"
						drop := "DROP TRIGGER snapshot_revision_inbounds_update"
						if dialect == DialectPostgres {
							query = "SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgname = 'snapshot_revision_inbounds' AND tgrelid = 'inbounds'::regclass"
							drop = "DROP TRIGGER snapshot_revision_inbounds ON inbounds"
						}
						if err := db.Raw(query).Scan(&definition).Error; err != nil {
							b.Fatal(err)
						}
						definition = strings.Replace(definition, "UPDATE ON", "UPDATE OF settings ON", 1)
						if err := db.Exec(drop).Error; err != nil {
							b.Fatal(err)
						}
						if err := db.Exec(definition).Error; err != nil {
							b.Fatal(err)
						}
					}
					for i := range 8 {
						if err := db.Create(&model.Inbound{Tag: fmt.Sprintf("catalog-%d", i), Settings: string(settings)}).Error; err != nil {
							b.Fatal(err)
						}
					}
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						if err := db.Exec("UPDATE inbounds SET up = up + 1, down = down + 1").Error; err != nil {
							b.Fatal(err)
						}
					}
					b.ReportMetric(8, "rows/op")
					b.ReportMetric(float64(8*len(settings)), "config-bytes/op")
				})
			}
		})
	}
}
