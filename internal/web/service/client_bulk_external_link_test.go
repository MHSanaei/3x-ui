package service

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBulkAppendExternalLinksPreservesExistingRowsAndSkipsDuplicates(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	clients := []model.ClientRecord{{Email: "alice"}, {Email: "bob"}, {Email: "unselected"}}
	if err := db.Create(&clients).Error; err != nil {
		t.Fatal(err)
	}
	existing := []model.ClientExternalLink{
		{ClientId: clients[0].Id, Kind: "link", Value: "trojan://pw@old.example:443", SortIndex: 7, Remark: "keep", Enable: new(false), ExpiryTime: 123, NamePrefix: "prefix", LastFetchAt: 456, LastFetchError: "timeout"},
		{ClientId: clients[0].Id, Kind: "subscription", Value: "https://provider.example/sub", SortIndex: 12, Remark: "original", Enable: new(true)},
		{ClientId: clients[2].Id, Kind: "link", Value: "trojan://pw@old.example:443", SortIndex: 99, Enable: new(true)},
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	inputs := []ExternalLinkInput{
		{Kind: "subscription", Value: " https://provider.example/sub ", Remark: "new", Enable: new(false), ExpiryTime: 789, NamePrefix: "provider "},
		{Kind: "link", Value: "vless://uuid@new.example:443", Remark: "backup"},
		{Kind: "link", Value: "vless://uuid@new.example:443", Remark: "duplicate must not replace"},
	}
	svc := &ClientService{}
	result, err := svc.BulkAppendExternalLinks([]string{" alice ", "bob", "alice", "missing"}, inputs)
	if err != nil {
		t.Fatal(err)
	}
	wantResult := BulkAdjustResult{Adjusted: 2, Skipped: []BulkAdjustReport{{Email: "missing", Reason: "client not found"}}, ExternalLinks: &BulkExternalLinksResult{Added: 3, Skipped: 3, Errors: 1}}
	if !reflect.DeepEqual(result, wantResult) {
		t.Fatalf("result = %+v, want %+v", result, wantResult)
	}
	var after []model.ClientExternalLink
	if err := db.Where("id IN ?", []int{existing[0].Id, existing[1].Id, existing[2].Id}).Order("id").Find(&after).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, existing) {
		t.Fatalf("existing rows changed: %+v, want %+v", after, existing)
	}
	alice, err := svc.GetExternalLinksForRecord(clients[0].Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(alice) != 3 || alice[2].Value != inputs[1].Value || alice[2].SortIndex != 13 || alice[2].Remark != "backup" {
		t.Fatalf("alice links = %+v", alice)
	}
	bob, err := svc.GetExternalLinksForRecord(clients[1].Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(bob) != 2 || bob[0].Kind != "subscription" || bob[0].Value != strings.TrimSpace(inputs[0].Value) || bob[0].Remark != "new" || *bob[0].Enable || bob[0].ExpiryTime != 789 || bob[0].NamePrefix != "provider " || bob[0].SortIndex != 0 || bob[1].SortIndex != 1 {
		t.Fatalf("bob links = %+v", bob)
	}
	repeated, err := svc.BulkAppendExternalLinks([]string{"alice", "bob"}, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Adjusted != 0 || *repeated.ExternalLinks != (BulkExternalLinksResult{Skipped: 6}) {
		t.Fatalf("repeat = %+v", repeated)
	}
	var count int64
	if err := db.Model(&model.ClientExternalLink{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Fatalf("row count = %d, want 6", count)
	}
}

func TestBulkAppendExternalLinksValidatesAllInputsBeforeWriting(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input ExternalLinkInput
		want  string
	}{
		{"link", ExternalLinkInput{Kind: "link", Value: "https://wrong.example"}, "unsupported or invalid share link: https://wrong.example\n"},
		{"subscription", ExternalLinkInput{Kind: "subscription", Value: "ftp://wrong.example"}, "external subscription must be an http(s) URL: ftp://wrong.example\n"},
		{"kind", ExternalLinkInput{Kind: "unknown", Value: "value"}, "unknown external link kind: unknown\n"},
		{"expiry", ExternalLinkInput{Kind: "subscription", Value: "https://provider.example/sub", ExpiryTime: -1}, "external link expiryTime must be 0 (never) or a future unix millisecond timestamp: https://provider.example/sub\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupBulkDB(t)
			rec := model.ClientRecord{Email: "alice"}
			if err := database.GetDB().Create(&rec).Error; err != nil {
				t.Fatal(err)
			}
			result, err := (&ClientService{}).BulkAppendExternalLinks([]string{"alice"}, []ExternalLinkInput{{Kind: "link", Value: "trojan://pw@valid.example:443"}, tt.input})
			if err == nil || err.Error() != tt.want || result.Adjusted != 0 {
				t.Fatalf("result/error = %+v/%v, want %q", result, err, tt.want)
			}
			var count int64
			if err := database.GetDB().Model(&model.ClientExternalLink{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("invalid batch wrote %d rows", count)
			}
		})
	}
}

func TestBulkAppendExternalLinksRejectsEmptySelectionAndRows(t *testing.T) {
	setupBulkDB(t)
	for _, tt := range []struct {
		name   string
		emails []string
		inputs []ExternalLinkInput
		want   string
	}{
		{"no links", []string{"alice"}, nil, "at least one external link or subscription is required\n"},
		{"blank rows", []string{"alice"}, []ExternalLinkInput{{Kind: "link", Value: " \t"}}, "at least one external link or subscription is required\n"},
		{"empty selection", []string{"", "  "}, []ExternalLinkInput{{Kind: "link", Value: "trojan://pw@example.com:443"}}, "client emails are required\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := (&ClientService{}).BulkAppendExternalLinks(tt.emails, tt.inputs)
			if err == nil || err.Error() != tt.want || result.Adjusted != 0 || *result.ExternalLinks != (BulkExternalLinksResult{}) {
				t.Fatalf("result/error = %+v/%v, want no writes and %q", result, err, tt.want)
			}
		})
	}
	result, err := (&ClientService{}).BulkAppendExternalLinks([]string{"missing"}, []ExternalLinkInput{{Kind: "link", Value: "trojan://pw@example.com:443"}})
	want := BulkAdjustResult{Skipped: []BulkAdjustReport{{Email: "missing", Reason: "client not found"}}, ExternalLinks: &BulkExternalLinksResult{Errors: 1}}
	if err != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("all missing result/error = %+v/%v, want %+v", result, err, want)
	}
}

func TestBulkAppendExternalLinksRollsBackEarlierInsertBatches(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	clients := []model.ClientRecord{{Email: "alice"}, {Email: "bob"}}
	if err := db.Create(&clients).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(fmt.Sprintf(`CREATE TRIGGER reject_bulk_link BEFORE INSERT ON client_external_links WHEN NEW.client_id = %d AND NEW.value = 'trojan://pw@server.example:443#119' BEGIN SELECT RAISE(ABORT, 'injected link failure'); END`, clients[1].Id)).Error; err != nil {
		t.Fatal(err)
	}
	inputs := make([]ExternalLinkInput, 120)
	for i := range inputs {
		inputs[i] = ExternalLinkInput{Kind: "link", Value: fmt.Sprintf("trojan://pw@server.example:443#%d", i)}
	}
	result, err := (&ClientService{}).BulkAppendExternalLinks([]string{"alice", "bob"}, inputs)
	if err == nil || !strings.Contains(err.Error(), "injected link failure") || result.Adjusted != 0 || result.ExternalLinks.Added != 0 {
		t.Fatalf("rollback result/error = %+v/%v", result, err)
	}
	var count int64
	if err := db.Model(&model.ClientExternalLink{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rollback left %d rows", count)
	}
}

func TestBulkAppendExternalLinksBatchesReadsAndConcurrentRetries(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	clients := make([]model.ClientRecord, 60)
	emails := make([]string, len(clients))
	for i := range clients {
		emails[i] = fmt.Sprintf("client-%03d", i)
		clients[i].Email = emails[i]
	}
	if err := db.Create(&clients).Error; err != nil {
		t.Fatal(err)
	}
	queries := 0
	if err := db.Callback().Query().After("gorm:query").Register("count_bulk_link_reads", func(tx *gorm.DB) { queries++ }); err != nil {
		t.Fatal(err)
	}
	input := []ExternalLinkInput{{Kind: "subscription", Value: "https://provider.example/sub"}}
	result, err := (&ClientService{}).BulkAppendExternalLinks(emails, input)
	if err := db.Callback().Query().Remove("count_bulk_link_reads"); err != nil {
		t.Fatal(err)
	}
	if err != nil || result.ExternalLinks.Added != 60 || queries != 2 {
		t.Fatalf("result/error/reads = %+v/%v/%d, want added=60 and 2 reads", result, err, queries)
	}
	var wg sync.WaitGroup
	results := make(chan BulkAdjustResult, 2)
	errors := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			res, err := (&ClientService{}).BulkAppendExternalLinks(emails, []ExternalLinkInput{{Kind: "link", Value: "trojan://pw@new.example:443"}})
			results <- res
			errors <- err
		})
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	added, skipped := 0, 0
	for res := range results {
		added += res.ExternalLinks.Added
		skipped += res.ExternalLinks.Skipped
	}
	if added != 60 || skipped != 60 {
		t.Fatalf("concurrent results: added=%d skipped=%d, want 60/60", added, skipped)
	}
}

func TestBulkAppendExternalLinksCrossesClientQueryChunks(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	clients := make([]model.ClientRecord, sqlInChunk+1)
	emails := make([]string, len(clients))
	for i := range clients {
		emails[i] = fmt.Sprintf("chunk-client-%04d", i)
		clients[i].Email = emails[i]
	}
	if err := db.CreateInBatches(&clients, sqlInChunk).Error; err != nil {
		t.Fatal(err)
	}
	queries := 0
	if err := db.Callback().Query().After("gorm:query").Register("count_chunk_link_reads", func(_ *gorm.DB) { queries++ }); err != nil {
		t.Fatal(err)
	}
	result, err := (&ClientService{}).BulkAppendExternalLinks(emails, []ExternalLinkInput{{Kind: "subscription", Value: "https://provider.example/sub"}})
	if err := db.Callback().Query().Remove("count_chunk_link_reads"); err != nil {
		t.Fatal(err)
	}
	want := BulkAdjustResult{Adjusted: len(clients), ExternalLinks: &BulkExternalLinksResult{Added: len(clients)}}
	if err != nil || !reflect.DeepEqual(result, want) || queries != 4 {
		t.Fatalf("chunked result/error/reads = %+v/%v/%d, want %+v and 4 reads", result, err, queries, want)
	}
	var rows []model.ClientExternalLink
	if err := db.Order("client_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(clients) || rows[len(rows)-1].ClientId != clients[len(clients)-1].Id {
		t.Fatalf("chunked rows = %d, last = %+v, want links for every selected client", len(rows), rows[len(rows)-1])
	}
}
