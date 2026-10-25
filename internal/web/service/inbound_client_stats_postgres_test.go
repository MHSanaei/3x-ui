package service

import (
	"os"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
)

func TestClientStatsIdentitiesPostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires XUI_DB_TYPE=postgres and XUI_DB_DSN")
	}
	cleanup, err := testpg.IsolatePackage("stats_identities")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"batch", TestClientStatsIdentitiesBatchesPreloadedInbounds},
		{"chunk_boundary", TestBackfillClientStatsChunkBoundaryAndEmpty},
		{"projection_failure", TestBackfillClientStatsProjectionFailurePreservesStats},
		{"traffic_page_failure", TestBackfillClientStatsTrafficPageFailureKeepsFreshPreload},
		{"sibling_and_identity", TestEnrichClientStatsPreservesSiblingTrafficAndClientOrder},
		{"successive_identity", TestEnrichClientStats_UsesClientsTableUUIDWhenSettingsStale},
		{"transaction", TestClientStatsIdentitiesUsesSuppliedTransaction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseCleanup, err := testpg.IsolatePackage("stats_case")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(caseCleanup)
			tc.run(t)
		})
	}
}
