package service

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAttachCandidatesSelection(t *testing.T) {
	setupBulkDB(t)
	testAttachCandidatesSelection(t)
}

func TestAttachCandidatesSelectionPostgres(t *testing.T) {
	durablePostgresDB(t)
	testAttachCandidatesSelection(t)
}

func testAttachCandidatesSelection(t *testing.T) {
	t.Helper()
	db := database.GetDB()
	ctx := context.Background()
	svc := &ClientService{}
	suffix := time.Now().UnixNano()
	source := durableTestInbound(nil, fmt.Sprintf("attach-source-%d", suffix), 42541)
	target := durableTestInbound(nil, fmt.Sprintf("attach-target-%d", suffix), 42542)
	if err := db.Create(source).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatal(err)
	}
	email := func(i int) string { return fmt.Sprintf("candidate-%d-%04d@x", suffix, i) }
	clients := make([]model.Client, 121)
	for i := range clients {
		clients[i] = model.Client{
			Email: email(i), SubID: fmt.Sprintf("test%d", i), Enable: i != 2, Group: "fixture",
			ID: fmt.Sprintf("11111111-1111-4111-8111-%012d", i),
		}
	}
	clients[119].Comment = "Журнал match_% only"
	keepAlive := model.KeepAliveValue("19")
	clients[3].KeepAlive = &keepAlive
	if err := svc.SyncInbound(nil, source.Id, clients); err != nil {
		t.Fatal(err)
	}
	if err := svc.SyncInbound(nil, target.Id, clients[:1]); err != nil {
		t.Fatal(err)
	}
	p := AttachCandidateParams{SourceID: source.Id, Targets: strconv.Itoa(target.Id)}
	first, err := svc.ListAttachCandidates(ctx, p)
	if err != nil || len(first.Items) != 100 || first.Total != 121 || first.Eligible != 120 || !first.Items[0].AlreadyAttached || first.Items[2].Enable {
		t.Fatalf("candidate first page = %+v, err=%v", first, err)
	}
	p.Page = 2
	second, err := svc.ListAttachCandidates(ctx, p)
	if err != nil || len(second.Items) != 21 {
		t.Fatalf("second page size = %v, err=%v", second, err)
	}
	search, err := svc.ListAttachCandidates(ctx, AttachCandidateParams{SourceID: source.Id, Targets: p.Targets, Search: "журнал MATCH_%"})
	if err != nil || len(search.Items) != 1 || search.Items[0].Email != email(119) {
		t.Fatalf("literal source search = %+v, err=%v", search, err)
	}
	group, err := svc.ListAttachCandidates(ctx, AttachCandidateParams{Targets: p.Targets, Group: "fixture", Search: fmt.Sprintf("candidate-%d", suffix)})
	if err != nil || group.Total != 121 || !slices.Contains(group.Groups, "fixture") {
		t.Fatalf("group search = %+v, err=%v", group, err)
	}

	p.Freeze = true
	p.Search = "ignored"
	scope, err := svc.ListAttachCandidates(ctx, p)
	if err != nil || scope.Eligible != 120 || len(scope.Items) != 0 || scope.ScopeToken == "" {
		t.Fatalf("freeze = %+v, err=%v", scope, err)
	}
	inboundSvc := &InboundService{}
	request := AttachSelection{
		Mode: "allMatching", SourceID: source.Id, ScopeToken: scope.ScopeToken,
		ExcludedClientIDs: []int{first.Items[1].ID, second.Items[0].ID},
	}
	extra := model.Client{Email: email(121), Enable: true}
	if err := svc.ApplyInboundClientDelta(nil, source.Id, []model.Client{extra}, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyInboundClientDelta(nil, source.Id, nil, []string{clients[2].Email}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.BulkAttachSelection(ctx, inboundSvc, []int{target.Id}, request); err == nil {
		t.Fatal("a client that leaves the frozen scope should invalidate the selection")
	}
	var linked int64
	if err := db.Model(&model.ClientInbound{}).Where("inbound_id = ?", target.Id).Count(&linked).Error; err != nil || linked != 1 {
		t.Fatalf("stale selection mutated target: links=%d err=%v", linked, err)
	}
	if err := svc.ApplyInboundClientDelta(nil, source.Id, clients[2:3], nil); err != nil {
		t.Fatal(err)
	}
	result, _, err := svc.BulkAttachSelection(ctx, inboundSvc, []int{target.Id}, request)
	if err != nil || result.Attached != 118 || result.Skipped != 0 || result.Errors != 0 {
		t.Fatalf("scoped attach = %+v, err=%v", result, err)
	}
	got, err := svc.ListForInbound(nil, target.Id)
	if err != nil || len(got) != 119 {
		t.Fatalf("target has %d clients, err=%v", len(got), err)
	}
	for _, row := range got {
		if row.Email == first.Items[1].Email || row.Email == second.Items[0].Email || row.Email == extra.Email ||
			(row.Email == email(2) && row.Enable) {
			t.Fatalf("invalid attachment %+v", row)
		}
	}
	reloaded, err := inboundSvc.GetInbound(target.Id)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := inboundSvc.GetClients(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(configured, func(c model.Client) bool { return c.Email == email(3) })
	if i < 0 || configured[i].KeepAlive == nil || *configured[i].KeepAlive != keepAlive {
		t.Fatal("bulk attachment discarded the stored client keepalive")
	}
	explicit := AttachSelection{Mode: "explicit", SourceID: source.Id, ClientIDs: []int{first.Items[1].ID}}
	result, _, err = svc.BulkAttachSelection(ctx, inboundSvc, []int{target.Id}, explicit)
	if err != nil || result.Attached != 1 {
		t.Fatalf("explicit attachment = %+v, err=%v", result, err)
	}
}

func BenchmarkAttachCandidateScale(b *testing.B) {
	for _, count := range []int{1000, 10000, 50000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			setupScaleDB(b)
			ds := seedScaleDataset(b, count, 1)
			db := database.GetDB()
			target := durableTestInbound(nil, "attach-scale", 42543)
			if err := db.Create(target).Error; err != nil {
				b.Fatal(err)
			}
			svc := &ClientService{}
			for _, mode := range []string{"page", "search", "freeze"} {
				b.Run(mode, func(b *testing.B) {
					p := AttachCandidateParams{SourceID: ds.inboundIds[0], Targets: strconv.Itoa(target.Id), Freeze: mode == "freeze"}
					if mode == "search" {
						p.Search = ds.emails[count/2]
					}
					b.ReportAllocs()
					for b.Loop() {
						result, err := svc.ListAttachCandidates(context.Background(), p)
						if err != nil || len(result.Items) > attachCandidatePageSize || result.Eligible != int64(count) {
							b.Fatalf("bounded candidate page or eligibility count failed: %v", err)
						}
					}
				})
			}
		})
	}
}

func TestAttachTenThousandClientsScale(t *testing.T) {
	setupScaleDB(t)
	ds := seedScaleDataset(t, 10_000, 1)
	db := database.GetDB()
	target := durableTestInbound(nil, "attach-ten-thousand", 42543)
	if err := db.Create(target).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	svc := &ClientService{}
	scope, err := svc.ListAttachCandidates(ctx, AttachCandidateParams{
		SourceID: ds.inboundIds[0], Targets: strconv.Itoa(target.Id), Freeze: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var excluded []int
	excludedEmails := []string{ds.emails[0], ds.emails[5000], ds.emails[9999]}
	if err := db.Model(&model.ClientRecord{}).Where("email IN ?", excludedEmails).Pluck("id", &excluded).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	result, _, err := svc.BulkAttachSelection(ctx, &InboundService{}, []int{target.Id}, AttachSelection{
		Mode: "allMatching", SourceID: ds.inboundIds[0], ScopeToken: scope.ScopeToken, ExcludedClientIDs: excluded,
	})
	if err != nil || result.Attached != 9997 || result.Skipped != 0 || result.Errors != 0 {
		t.Fatalf("large attachment = %+v, err=%v", result, err)
	}
	t.Logf("attached 9997 of 10000 clients in %v", time.Since(start))
	var links, unexpected int64
	query := db.Model(&model.ClientInbound{}).Where("inbound_id = ?", target.Id)
	if err := query.Count(&links).Error; err != nil || links != 9997 {
		t.Fatalf("target links = %d, err=%v", links, err)
	}
	if err := query.Where("client_id IN ?", excluded).Count(&unexpected).Error; err != nil || unexpected != 0 {
		t.Fatalf("target has %d excluded clients, err=%v", unexpected, err)
	}
}
