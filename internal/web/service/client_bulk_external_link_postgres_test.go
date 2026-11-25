package service

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBulkAppendExternalLinks_PostgresCommitFailureAndConcurrentRetries(t *testing.T) {
	db := durablePostgresDB(t)
	emails := make([]string, sqlInChunk+1)
	clients := make([]model.ClientRecord, len(emails))
	for i := range clients {
		emails[i] = fmt.Sprintf("bulk-append-%04d@pg", i)
		clients[i].Email = emails[i]
	}
	cleanup := func() {
		if err := db.Exec("DELETE FROM client_external_links WHERE client_id IN (SELECT id FROM clients WHERE email IN ?)", emails).Error; err != nil {
			t.Errorf("clean external links: %v", err)
		}
		if err := db.Where("email IN ?", emails).Delete(&model.ClientRecord{}).Error; err != nil {
			t.Errorf("clean clients: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if err := db.CreateInBatches(&clients, sqlInChunk).Error; err != nil {
		t.Fatal(err)
	}
	results := make(chan BulkAdjustResult, 2)
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			result, err := (&ClientService{}).BulkAppendExternalLinks(emails, []ExternalLinkInput{{Kind: "subscription", Value: "https://provider.example/sub"}})
			results <- result
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
	for result := range results {
		added += result.ExternalLinks.Added
		skipped += result.ExternalLinks.Skipped
	}
	if added != len(clients) || skipped != len(clients) {
		t.Fatalf("concurrent append: added=%d skipped=%d, want %d/%d", added, skipped, len(clients), len(clients))
	}
	installDeferredCommitFailure(t, db, "create", "bulk:external_link_commit_failure", "client_external_links", "bulk_link_parent", "bulk_link_child")
	result, err := (&ClientService{}).BulkAppendExternalLinks(emails[:1], []ExternalLinkInput{{Kind: "link", Value: "trojan://pw@example.com:443"}})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "foreign key") || result.Adjusted != 0 || result.ExternalLinks.Added != 0 {
		t.Fatalf("commit result/error = %+v/%v, want zero committed rows and deferred foreign-key failure", result, err)
	}
	var rows []model.ClientExternalLink
	if err := db.Where("client_id IN (SELECT id FROM clients WHERE email IN ?)", emails).Order("client_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(clients) || rows[0].Kind != "subscription" || rows[len(rows)-1].Kind != "subscription" {
		t.Fatalf("rollback changed existing links: %+v", rows)
	}
}
