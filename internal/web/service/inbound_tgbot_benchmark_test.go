package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Keep the pre-fix implementation here so both paths run against identical data.
func (s *InboundService) getClientTrafficTgBotBefore6886(tgId int64) ([]*xray.ClientTraffic, error) {
	db := database.GetDB()

	idQuery := fmt.Sprintf(
		"SELECT DISTINCT inbounds.id %s WHERE %s = ?",
		database.JSONClientsFromInbound(),
		database.JSONFieldText("client.value", "tgId"),
	)
	var inboundIds []int
	if err := db.Raw(idQuery, strconv.FormatInt(tgId, 10)).Scan(&inboundIds).Error; err != nil {
		logger.Errorf("Error retrieving inbounds with tgId %d: %v", tgId, err)
		return nil, err
	}

	var inbounds []*model.Inbound
	if len(inboundIds) > 0 {
		err := db.Model(model.Inbound{}).Where("id IN ?", inboundIds).Find(&inbounds).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Errorf("Error retrieving inbounds with tgId %d: %v", tgId, err)
			return nil, err
		}
	}

	var emails []string
	for _, inbound := range inbounds {
		clients, err := s.GetClients(inbound)
		if err != nil {
			logger.Errorf("Error retrieving clients for inbound %d: %v", inbound.Id, err)
			continue
		}
		for _, client := range clients {
			if client.TgID == tgId {
				emails = append(emails, client.Email)
			}
		}
	}

	// Chunked to stay under SQLite's bind-variable limit when a single Telegram
	// account owns thousands of clients across inbounds.
	uniqEmails := uniqueNonEmptyStrings(emails)
	traffics := make([]*xray.ClientTraffic, 0, len(uniqEmails))
	for _, batch := range chunkStrings(uniqEmails, sqliteMaxVars) {
		var page []*xray.ClientTraffic
		if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			logger.Errorf("Error retrieving ClientTraffic for emails %v: %v", batch, err)
			return nil, err
		}
		traffics = append(traffics, page...)
	}
	if len(traffics) == 0 {
		// Exclude warning I/O from the comparison of empty lookups.
		return nil, nil
	}

	// Populate UUID and other client data for each traffic record
	for i := range traffics {
		if ct, client, e := s.GetClientByEmail(traffics[i].Email); e == nil && ct != nil && client != nil {
			traffics[i].Enable = client.Enable
			traffics[i].UUID = client.ID
			traffics[i].SubId = client.SubID
		}
	}

	return traffics, nil
}

type tgBenchmarkQueries struct {
	gormlogger.Interface
	count int
}

func (l *tgBenchmarkQueries) Trace(_ context.Context, _ time.Time, _ func() (string, int64), _ error) {
	l.count++
}

func BenchmarkGetClientTrafficTgBot(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			dbtest.InitDB(b, filepath.Join(b.TempDir(), "x-ui.db"))
			db := database.GetDB()
			prefix := fmt.Sprintf("bench6886-%d-", time.Now().UnixNano())
			const targetID = int64(6886)
			records := make([]model.ClientRecord, size)
			traffics := make([]xray.ClientTraffic, size)
			clients := make([]model.Client, size)
			var links []model.ClientInbound
			var inboundIDs []int
			for i := range size {
				tgID := int64(100000 + i)
				if i%(size/4) == 0 {
					tgID = targetID
				}
				clients[i] = model.Client{Email: fmt.Sprintf("%s%05d", prefix, i), ID: fmt.Sprintf("uuid-%d", i), SubID: fmt.Sprintf("sub-%d", i), TgID: tgID, Enable: true}
				records[i] = *clients[i].ToRecord()
				traffics[i] = xray.ClientTraffic{Email: clients[i].Email, Enable: true, Up: 10, Down: 20}
			}
			if err := db.CreateInBatches(&records, 100).Error; err != nil {
				b.Fatal(err)
			}
			for i := range 8 {
				start, end := i*size/8, (i+1)*size/8
				settings, err := json.Marshal(struct {
					Clients []model.Client `json:"clients"`
				}{clients[start:end]})
				if err != nil {
					b.Fatal(err)
				}
				ib := &model.Inbound{UserId: 1, Tag: fmt.Sprintf("%s%d", prefix, i), Port: 41900 + i, Protocol: model.VLESS, Enable: true, Settings: string(settings)}
				if err := db.Create(ib).Error; err != nil {
					b.Fatal(err)
				}
				inboundIDs = append(inboundIDs, ib.Id)
				for j := start; j < end; j++ {
					links = append(links, model.ClientInbound{ClientId: records[j].Id, InboundId: ib.Id})
					traffics[j].InboundId = ib.Id
				}
			}
			if err := db.CreateInBatches(&links, 100).Error; err != nil {
				b.Fatal(err)
			}
			if err := db.CreateInBatches(&traffics, 100).Error; err != nil {
				b.Fatal(err)
			}
			if database.IsPostgres() {
				// Bulk-loaded synthetic tables need statistics before comparing plans.
				for _, table := range []string{"clients", "client_inbounds", "client_traffics", "inbounds"} {
					if err := db.Exec("ANALYZE " + table).Error; err != nil {
						b.Fatal(err)
					}
				}
			}
			b.Cleanup(func() {
				_ = db.Where("inbound_id IN ?", inboundIDs).Delete(&model.ClientInbound{}).Error
				_ = db.Where("email LIKE ?", prefix+"%").Delete(&xray.ClientTraffic{}).Error
				_ = db.Where("email LIKE ?", prefix+"%").Delete(&model.ClientRecord{}).Error
				_ = db.Where("id IN ?", inboundIDs).Delete(&model.Inbound{}).Error
			})
			svc := &InboundService{}
			before, err := svc.getClientTrafficTgBotBefore6886(targetID)
			if err != nil {
				b.Fatal(err)
			}
			after, err := svc.GetClientTrafficTgBot(targetID)
			if err != nil {
				b.Fatal(err)
			}
			if len(after) != 4 || !reflect.DeepEqual(before, after) {
				b.Fatalf("old/new semantics differ: before=%+v after=%+v", before, after)
			}
			if database.IsPostgres() && size == 10000 {
				trace := &tgLookupSQL{Interface: gormlogger.Default}
				previous := db.Logger
				db.Logger = trace
				_, err := svc.GetClientTrafficTgBot(targetID)
				db.Logger = previous
				if err != nil || len(trace.queries) != 1 {
					b.Fatalf("capture indexed SQL: %v, queries=%v", err, trace.queries)
				}
				oldSQL := fmt.Sprintf("SELECT DISTINCT inbounds.id %s WHERE %s = '%d'", database.JSONClientsFromInbound(), database.JSONFieldText("client.value", "tgId"), targetID)
				for _, query := range []string{oldSQL, trace.queries[0]} {
					var plan []struct {
						Line string `gorm:"column:QUERY PLAN"`
					}
					if err := db.Raw("EXPLAIN (ANALYZE, BUFFERS, TIMING OFF) " + query).Scan(&plan).Error; err != nil {
						b.Fatal(err)
					}
					for _, line := range plan {
						b.Log(line.Line)
					}
				}
			}
			for _, workload := range []struct {
				name string
				id   int64
			}{{"hit", targetID}, {"miss", 99999999}} {
				b.Run(workload.name, func(b *testing.B) {
					for _, impl := range []struct {
						name   string
						lookup func(int64) ([]*xray.ClientTraffic, error)
					}{{"old", svc.getClientTrafficTgBotBefore6886}, {"indexed", svc.GetClientTrafficTgBot}} {
						b.Run(impl.name, func(b *testing.B) {
							counter := &tgBenchmarkQueries{Interface: gormlogger.Default}
							previous := db.Logger
							db.Logger = counter
							defer func() { db.Logger = previous }()
							b.ReportAllocs()
							b.ResetTimer()
							for range b.N {
								rows, err := impl.lookup(workload.id)
								if err != nil {
									b.Fatal(err)
								}
								want := 4
								if workload.name == "miss" {
									want = 0
								}
								if len(rows) != want {
									b.Fatalf("got %d rows, want %d", len(rows), want)
								}
							}
							b.StopTimer()
							b.ReportMetric(float64(counter.count)/float64(b.N), "queries/op")
						})
					}
				})
			}
		})
	}
}
