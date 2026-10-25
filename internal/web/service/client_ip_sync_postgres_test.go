package service

import (
	"os"
	"strings"
	"testing"
)

func TestClientIpSyncPostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run PostgreSQL IP sync regressions")
	}
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"seeded snapshot", TestClientIpSyncSeededSnapshot},
		{"duplicates", TestClientIpSyncDuplicatePayloadsKeepUnion},
		{"raw snapshot retry", TestClientIpSyncRawSnapshotAndSingleRetry},
		{"create conflict", TestClientIpSyncCreateConflictMergesWinner},
		{"exhaustion rollback", TestClientIpSyncCASExhaustionRollsBack},
		{"concurrent writer", TestMergeInboundClientIps_RetriesAfterConcurrentWriter},
		{"attribution unchanged", TestNodeClientIpCanonicalUnchangedSkipsUpsert},
		{"remote ids", TestMergeInboundClientIps_CreatesNodeOnlyRowIgnoringRemoteId},
		{"attribution round trip", TestRecordLocalClientIps_RoundTripByGuid},
		{"attribution expiry", TestRecordLocalClientIps_MergesAndDropsStale},
		{"attribution empty", TestUpsertNodeClientIps_EmptyMergeDeletesRow},
		{"retention", TestPruneStaleClientIpsExpiresUnobservedRows},
		{"cloned GUID", TestMergeClientIpsByGuid_RemapsClonedNodeSubtree},
		{"maximum timestamps", TestMergeInboundClientIps_DedupKeepsMaxTimestamp},
		{"stale merge", TestMergeInboundClientIps_DropsStaleIps},
		{"stale insert", TestMergeInboundClientIps_SkipsAllStaleCreate},
	} {
		t.Run(tc.name, tc.run)
	}
}
