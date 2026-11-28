package panel

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func BenchmarkApiTokenMatchToken(b *testing.B) {
	dbtest.InitDB(b, filepath.Join(b.TempDir(), "token_bench.db"))
	svc := &ApiTokenService{}
	created, err := svc.Create("bench-token", model.ApiScopeAdmin, 0)
	if err != nil {
		b.Fatalf("Create: %v", err)
	}
	plain := created.Token

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, ok := svc.MatchToken(plain); !ok {
			b.Fatal("MatchToken = false")
		}
	}
}
