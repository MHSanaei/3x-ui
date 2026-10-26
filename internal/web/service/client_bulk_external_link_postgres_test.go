package service

import (
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBulkAppendExternalLinks_PostgresCommitFailureAndConcurrentRetries(t *testing.T) {
	db := durablePostgresDB(t)
	emails := []string{"bulk-append-a@pg", "bulk-append-b@pg"}
	cleanup := func() {
		_ = db.Exec("DELETE FROM client_external_links WHERE client_id IN (SELECT id FROM client_records WHERE email IN ?)", emails).Error
		_ = db.Where("email IN ?", emails).Delete(&model.ClientRecord{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)
	clients := []model.ClientRecord{{Email: emails[0]}, {Email: emails[1]}}
	if err := db.Create(&clients).Error; err != nil {
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
	if added != 2 || skipped != 2 {
		t.Fatalf("concurrent append: added=%d skipped=%d, want 2/2", added, skipped)
	}
	installDeferredCommitFailure(t, db, "create", "bulk:external_link_commit_failure", "client_external_links", "bulk_link_parent", "bulk_link_child")
	result, err := (&ClientService{}).BulkAppendExternalLinks(emails, []ExternalLinkInput{{Kind: "link", Value: "trojan://pw@example.com:443"}})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "foreign key") || result.Adjusted != 0 || result.ExternalLinks.Added != 0 {
		t.Fatalf("commit result/error = %+v/%v, want zero committed rows and deferred foreign-key failure", result, err)
	}
	var rows []model.ClientExternalLink
	if err := db.Where("client_id IN ?", []int{clients[0].Id, clients[1].Id}).Order("client_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Kind != "subscription" || rows[1].Kind != "subscription" {
		t.Fatalf("rollback changed existing links: %+v", rows)
	}
}
