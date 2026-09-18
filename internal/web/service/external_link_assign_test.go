package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func boolPtr(v bool) *bool { return &v }

// seedAssignInbound seeds one more inbound into the DB the test already set up:
// the shared helper re-initialises it, which a two-inbound case cannot use.
func seedAssignInbound(t *testing.T, tag string, port int, clients []model.Client) *model.Inbound {
	t.Helper()
	seedInboundConflict(t, tag, "0.0.0.0", port, model.VLESS, `{"network":"tcp"}`, clientsSettings(t, clients))
	inbound := loadInboundByTag(t, tag)
	if err := (&ClientService{}).SyncInbound(nil, inbound.Id, clients); err != nil {
		t.Fatalf("SyncInbound(%s): %v", tag, err)
	}
	return inbound
}

func assignUUID(n int) string {
	return fmt.Sprintf("1f8d5e1c-3f3b-4d2a-9f7e-4b1c2d3e4f5%d", n)
}

// TestExternalLinkAssignExpandsEveryTargetKind is the assign path the page and
// the bot both use: one call naming a client, a group, an inbound, the whole
// panel and new clients must produce five distinct bindings, and each scope must
// still decide the resolution on its own - most specific first.
func TestExternalLinkAssignExpandsEveryTargetKind(t *testing.T) {
	setupConflictDB(t)
	first := seedAssignInbound(t, "assign-in-1", 22101, []model.Client{
		{Email: "assign-a@x", Group: "vip", ID: assignUUID(1), Enable: true},
		{Email: "assign-b@x", Group: "vip", ID: assignUUID(2), Enable: true},
	})
	second := seedAssignInbound(t, "assign-in-2", 22102, []model.Client{
		{Email: "assign-c@x", Group: "vip", ID: assignUUID(3), Enable: true},
		{Email: "assign-d@x", ID: assignUUID(4), Enable: true},
	})
	seedLibraryGroup(t, "vip")
	link := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign", "assign")

	affected, err := (&ClientService{}).ExternalLinkAssign(link.Id, ExternalLinkAssignRequest{
		Emails:     []string{"assign-a@x"},
		Group:      "vip",
		InboundId:  first.Id,
		Global:     true,
		NewClients: true,
		Enable:     boolPtr(true),
		NamePrefix: "pref-",
	})
	if err != nil {
		t.Fatalf("assign to every target kind: %v", err)
	}
	if affected != 5 {
		t.Fatalf("affected = %d, want 5 (client, group, inbound, global, new clients)", affected)
	}

	targets, err := (&ClientService{}).ExternalLinkTargets(link.Id)
	if err != nil {
		t.Fatalf("targets: %v", err)
	}
	got := make([]string, 0, len(targets))
	for _, target := range targets {
		got = append(got, target.TargetType+":"+target.Name)
	}
	want := []string{
		"client:assign-a@x",
		"global:all clients",
		"group:vip",
		"inbound:assign-in-1",
		"new_clients:new clients",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("targets = %v, want %v", got, want)
	}
	if second.Id == first.Id {
		t.Fatal("the two seeded inbounds collapsed into one; the scope case below would not prove anything")
	}

	// Each client is resolved by the most specific scope that reaches it, and the
	// request's override rides every binding it created, not just the client one.
	cases := []struct {
		email string
		scope string
	}{
		{"assign-a@x", model.ExternalLinkTargetClient},
		{"assign-b@x", model.ExternalLinkTargetInbound},
		{"assign-c@x", model.ExternalLinkTargetGroup},
		{"assign-d@x", model.ExternalLinkTargetGlobal},
	}
	for _, tc := range cases {
		resolved := resolveOneClient(t, lookupClientRecord(t, tc.email).Id)
		if len(resolved) != 1 {
			t.Errorf("%s resolves %d links, want 1: %+v", tc.email, len(resolved), resolved)
			continue
		}
		if resolved[0].Scope != tc.scope {
			t.Errorf("%s resolved through scope %q, want %q", tc.email, resolved[0].Scope, tc.scope)
		}
		if resolved[0].NamePrefix != "pref-" {
			t.Errorf("%s resolved through %s with namePrefix %q, want the request's pref-",
				tc.email, tc.scope, resolved[0].NamePrefix)
		}
	}
}

// TestExternalLinkAssignRefusesUnknownSelectors pins the error each selector
// produces: a typo must fail loudly rather than silently binding nothing.
func TestExternalLinkAssignRefusesUnknownSelectors(t *testing.T) {
	seedLibraryInbound(t, "assign-in-3", 22103, []model.Client{libraryClient("assign-e@x", "vip")})
	seedLibraryGroup(t, "vip")
	link := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-refuse", "refuse")

	cases := []struct {
		name   string
		linkId int
		req    ExternalLinkAssignRequest
		want   string
	}{
		{"unknown email", link.Id, ExternalLinkAssignRequest{Emails: []string{"ghost@x"}}, "client not found: ghost@x"},
		{"unknown inbound", link.Id, ExternalLinkAssignRequest{InboundId: 987654}, "inbound not found"},
		{"no target", link.Id, ExternalLinkAssignRequest{}, "no target given: pass emails, group, inboundId, global or newClients"},
		{"unknown link", 987654, ExternalLinkAssignRequest{Global: true}, "link not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&ClientService{}).ExternalLinkAssign(tc.linkId, tc.req)
			if err == nil {
				t.Fatalf("assign with %s succeeded, want %q", tc.name, tc.want)
			}
			if got := strings.TrimSpace(err.Error()); got != tc.want {
				t.Fatalf("error = %q, want %q", got, tc.want)
			}
		})
	}

	// Unassigning a group that does not exist must not create it, unlike assign.
	_, err := (&ClientService{}).ExternalLinkUnassign(link.Id, ExternalLinkAssignRequest{Group: "ghost-group"})
	if err == nil || strings.TrimSpace(err.Error()) != "group not found: ghost-group" {
		t.Fatalf("unassign unknown group error = %v, want group not found: ghost-group", err)
	}

	// An unknown group on assign is the documented exception: it is created, the
	// way bulkAdd creates one.
	affected, err := (&ClientService{}).ExternalLinkAssign(link.Id, ExternalLinkAssignRequest{Group: "brand-new"})
	if err != nil || affected != 1 {
		t.Fatalf("assign to a new group = (%d, %v), want (1, nil)", affected, err)
	}
	if got := groupRowId(t, "brand-new"); got == 0 {
		t.Fatal("assign to a new group did not create the group row")
	}
}

// TestExternalLinkAssignRefreshesInPlace pins that re-assigning the same target
// updates its overrides instead of stacking a second binding: two rows for one
// (link, target) would make the resolution depend on row order.
func TestExternalLinkAssignRefreshesInPlace(t *testing.T) {
	seedLibraryInbound(t, "assign-in-4", 22104, []model.Client{libraryClient("assign-f@x", "")})
	link := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-refresh", "refresh")
	clientId := lookupClientRecord(t, "assign-f@x").Id

	first, err := (&ClientService{}).ExternalLinkAssign(link.Id, ExternalLinkAssignRequest{
		Emails: []string{"assign-f@x"}, Enable: boolPtr(true), NamePrefix: "one-",
	})
	if err != nil || first != 1 {
		t.Fatalf("first assign = (%d, %v), want (1, nil)", first, err)
	}
	second, err := (&ClientService{}).ExternalLinkAssign(link.Id, ExternalLinkAssignRequest{
		Emails: []string{"assign-f@x"}, Enable: boolPtr(true), NamePrefix: "two-", ExpiryTime: 1893456000000,
	})
	if err != nil || second != 1 {
		t.Fatalf("second assign = (%d, %v), want (1, nil)", second, err)
	}
	assertAssignmentCount(t, link.Id, model.ExternalLinkTargetClient, clientId, 1)

	resolved := resolveOneClient(t, clientId)
	if len(resolved) != 1 {
		t.Fatalf("resolved %d links, want 1", len(resolved))
	}
	if resolved[0].NamePrefix != "two-" || resolved[0].ExpiryTime != 1893456000000 {
		t.Fatalf("refreshed binding = prefix %q expiry %d, want two- / 1893456000000",
			resolved[0].NamePrefix, resolved[0].ExpiryTime)
	}
}

// TestExternalLinkUnassignRemovesOnlyTheNamedTarget keeps the blast radius of an
// unassign honest: dropping a client's binding must not take the group's.
func TestExternalLinkUnassignRemovesOnlyTheNamedTarget(t *testing.T) {
	seedLibraryInbound(t, "assign-in-5", 22105, []model.Client{libraryClient("assign-g@x", "vip")})
	seedLibraryGroup(t, "vip")
	link := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-unassign", "unassign")
	clientId := lookupClientRecord(t, "assign-g@x").Id

	if _, err := (&ClientService{}).ExternalLinkAssign(link.Id, ExternalLinkAssignRequest{
		Emails: []string{"assign-g@x"}, Group: "vip", Enable: boolPtr(true),
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if len(resolveOneClient(t, clientId)) != 1 {
		t.Fatal("the client does not resolve the link after being bound directly")
	}

	deleted, err := (&ClientService{}).ExternalLinkUnassign(link.Id, ExternalLinkAssignRequest{Emails: []string{"assign-g@x"}})
	if err != nil || deleted != 1 {
		t.Fatalf("unassign client = (%d, %v), want (1, nil)", deleted, err)
	}
	if len(resolveOneClient(t, clientId)) != 1 {
		t.Fatal("dropping the client's own binding also dropped the group's")
	}

	deleted, err = (&ClientService{}).ExternalLinkUnassign(link.Id, ExternalLinkAssignRequest{Group: "vip"})
	if err != nil || deleted != 1 {
		t.Fatalf("unassign group = (%d, %v), want (1, nil)", deleted, err)
	}
	if got := resolveOneClient(t, clientId); len(got) != 0 {
		t.Fatalf("after both unassigns the client still resolves %+v", got)
	}

	deleted, err = (&ClientService{}).ExternalLinkUnassign(link.Id, ExternalLinkAssignRequest{Group: "vip"})
	if err != nil || deleted != 0 {
		t.Fatalf("unassigning a target with no binding = (%d, %v), want (0, nil)", deleted, err)
	}
}

// TestClientExternalLinkViewsCarryScopeAndFetchStatus covers the client card's
// Links tab: own row first with the scope that granted each one, plus the fetch
// outcome the subscription path stamped on the shared row.
func TestClientExternalLinkViewsCarryScopeAndFetchStatus(t *testing.T) {
	seedLibraryInbound(t, "assign-in-6", 22106, []model.Client{libraryClient("assign-h@x", "vip")})
	seedLibraryGroup(t, "vip")
	clientId := lookupClientRecord(t, "assign-h@x").Id
	url := "https://provider.example/assign-sub"

	own := seedLibraryLink(t, model.ExternalLinkKindSubscription, url, "own")
	group := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-group", "group")
	global := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-global", "global")
	assignLibraryLink(t, own.Id, model.ExternalLinkTargetClient, clientId, "")
	assignLibraryLink(t, group.Id, model.ExternalLinkTargetGroup, groupRowId(t, "vip"), "")
	assignLibraryLink(t, global.Id, model.ExternalLinkTargetGlobal, 0, "")

	if err := RecordExternalLinkFetchByValue(url, nil, errors.New("provider said 500")); err != nil {
		t.Fatalf("record a failed fetch: %v", err)
	}
	views, err := (&ClientService{}).ClientExternalLinkViews(clientId)
	if err != nil {
		t.Fatalf("views: %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("views = %d, want 3: %+v", len(views), views)
	}
	if views[0].LinkId != own.Id || !views[0].Own || views[0].AssignmentId == 0 {
		t.Fatalf("first view = %+v, want the client's own row with its assignment id", views[0])
	}
	if views[0].LastFetchError != "provider said 500" {
		t.Fatalf("own row fetch error = %q, want the recorded one", views[0].LastFetchError)
	}
	if views[0].LastFetchAt == 0 {
		t.Fatal("own row carries no last-fetch timestamp")
	}
	if views[1].Scope != model.ExternalLinkTargetGroup || views[2].Scope != model.ExternalLinkTargetGlobal {
		t.Fatalf("inherited scopes = %q, %q; want group then global", views[1].Scope, views[2].Scope)
	}

	// The successful expansion is stored for the restart-with-the-provider-down
	// path, so the round trip through the column has to survive.
	if err := RecordExternalLinkFetchByValue(url, []string{"trojan://fresh"}, nil); err != nil {
		t.Fatalf("record a good fetch: %v", err)
	}
	if got := LastExternalLinkLinksByValue(url); len(got) != 1 || got[0] != "trojan://fresh" {
		t.Fatalf("last links by value = %v, want [trojan://fresh]", got)
	}
	views, err = (&ClientService{}).ClientExternalLinkViews(clientId)
	if err != nil {
		t.Fatalf("views after a good fetch: %v", err)
	}
	if views[0].LastFetchError != "" {
		t.Fatalf("a successful fetch left the error %q on the row", views[0].LastFetchError)
	}
}

// TestApplyExternalLinkSyncIsAllOrNothing pins the transaction: a push carrying
// one value this panel refuses must not leave the other half applied.
func TestApplyExternalLinkSyncIsAllOrNothing(t *testing.T) {
	seedLibraryInbound(t, "assign-in-7", 22107, []model.Client{libraryClient("assign-i@x", "")})
	payload := &runtime.ExternalLinkSync{
		Links: []runtime.ExternalLinkSyncLink{
			{Kind: model.ExternalLinkKindLink, Value: "trojan://assign-valid", Enable: true},
			{Kind: "bogus-kind", Value: "trojan://assign-invalid"},
		},
	}
	if err := (&ClientService{}).ApplyExternalLinkSync(payload); err == nil {
		t.Fatal("a push carrying an unknown kind was accepted")
	}
	if rows := countRows(t, &model.ExternalLink{}); rows != 0 {
		t.Fatalf("library rows after a rejected push = %d, want 0: the valid half was committed", rows)
	}
	if rows := countRows(t, &model.ExternalLinkAssignment{}); rows != 0 {
		t.Fatalf("assignments after a rejected push = %d, want 0", rows)
	}
}

// TestPushExternalLinksSkipsANodeThatCannotTakeIt pins the guards the operator
// paths rely on: a disabled or offline node is skipped outright, so the edit
// never pays a round-trip whose answer could not arrive anyway.
func TestPushExternalLinksSkipsANodeThatCannotTakeIt(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)
	useTestRuntimeManager(t)
	f := newFakeNodeHTTP(t)
	ib := realNodeInbound(t, f, 53601, []model.Client{
		{Email: "skip@x", ID: "33333333-1111-2222-3333-444444444444", SubID: "sub-skip", Enable: true},
	})
	nodeId := *ib.NodeID
	link := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-skip", "skip")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetClient, lookupClientRecord(t, "skip@x").Id, "")

	cases := []struct {
		name   string
		status string
		enable bool
		want   int
	}{
		{"offline node", "offline", true, 0},
		{"disabled node", "online", false, 0},
		{"online node", "online", true, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := database.GetDB().Model(&model.Node{}).Where("id = ?", nodeId).
				Updates(map[string]any{"status": tc.status, "enable": tc.enable}).Error; err != nil {
				t.Fatalf("set node state: %v", err)
			}
			(&ClientService{}).PushExternalLinksForEmails(nodeId, []string{"skip@x"})
			if got := f.hitCount("/panel/api/links/sync"); got != tc.want {
				t.Fatalf("links/sync requests for a %s = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// TestExternalLinkLibraryDeleteTakesOnlyItsOwnBindings: deleting a library row
// must take its own bindings and nothing else.
func TestExternalLinkLibraryDeleteTakesOnlyItsOwnBindings(t *testing.T) {
	_, ids := seedLibraryInbound(t, "assign-in-8", 22108, []model.Client{libraryClient("assign-j@x", "vip")})
	seedLibraryGroup(t, "vip")
	clientId := ids["assign-j@x"]
	doomed := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-doomed", "doomed")
	kept := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-kept", "kept")
	assignLibraryLink(t, doomed.Id, model.ExternalLinkTargetClient, clientId, "")
	assignLibraryLink(t, doomed.Id, model.ExternalLinkTargetGroup, groupRowId(t, "vip"), "")
	assignLibraryLink(t, kept.Id, model.ExternalLinkTargetGlobal, 0, "")

	if err := (&ClientService{}).ExternalLinkLibraryDelete(doomed.Id); err != nil {
		t.Fatalf("delete the link: %v", err)
	}
	if rows := countRows(t, &model.ExternalLink{}); rows != 1 {
		t.Fatalf("library rows = %d, want the kept one only", rows)
	}
	assertAssignmentCount(t, doomed.Id, model.ExternalLinkTargetClient, clientId, 0)
	assertAssignmentCount(t, doomed.Id, model.ExternalLinkTargetGroup, groupRowId(t, "vip"), 0)
	assertAssignmentCount(t, kept.Id, model.ExternalLinkTargetGlobal, 0, 1)
	resolved := resolveOneClient(t, clientId)
	if len(resolved) != 1 || resolved[0].LinkId != kept.Id {
		t.Fatalf("the client resolves %+v, want only the kept link", resolved)
	}

	if err := (&ClientService{}).ExternalLinkLibraryDelete(doomed.Id); err == nil {
		t.Fatal("deleting a link that is already gone succeeded")
	}
}

// TestExternalLinkLibraryReorderKeepsOrderAndRefusesTheMastersRows pins both
// halves of the reorder path: the stored order, and the read-only guard a master
// row needs or the next push silently undoes the move.
func TestExternalLinkLibraryReorderKeepsOrderAndRefusesTheMastersRows(t *testing.T) {
	setupConflictDB(t)
	first := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-r1", "r1")
	second := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-r2", "r2")

	if err := (&ClientService{}).ExternalLinkLibraryReorder([]int{second.Id, first.Id}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	links, err := (&ClientService{}).ExternalLinkLibraryList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 2 || links[0].Id != second.Id || links[1].Id != first.Id {
		t.Fatalf("order after reorder = %v, want %d then %d", []int{links[0].Id, links[1].Id}, second.Id, first.Id)
	}

	if err := (&ClientService{}).ExternalLinkLibraryReorder([]int{987654}); err == nil {
		t.Fatal("reorder with an unknown id succeeded")
	}

	pushed := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-pushed", "pushed")
	if err := database.GetDB().Model(&model.ExternalLink{}).Where("id = ?", pushed.Id).
		Update("origin", model.ExternalLinkOriginNode).Error; err != nil {
		t.Fatalf("mark the row as master-pushed: %v", err)
	}
	if err := (&ClientService{}).ExternalLinkLibraryReorder([]int{first.Id, pushed.Id}); err == nil {
		t.Fatal("reorder accepted a row the master pushed")
	}
}

// TestPushExternalLinksWithoutARuntimeManagerIsANoOp: the manager is absent
// until the panel finishes booting, and a link saved in that window must not
// panic the save.
func TestPushExternalLinksWithoutARuntimeManagerIsANoOp(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)
	f := newFakeNodeHTTP(t)
	ib := realNodeInbound(t, f, 53701, []model.Client{
		{Email: "assign-k@x", ID: assignUUID(9), SubID: "sub-assign-k", Enable: true},
	})
	link := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://assign-no-mgr", "no-mgr")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetClient, lookupClientRecord(t, "assign-k@x").Id, "")

	prev := runtime.GetManager()
	runtime.SetManager(nil)
	t.Cleanup(func() { runtime.SetManager(prev) })

	(&ClientService{}).PushExternalLinksForEmails(*ib.NodeID, []string{"assign-k@x"})
	(&ClientService{}).PushExternalLinksToNode(context.Background(), nil)

	if got := f.hitCount("/panel/api/links/sync"); got != 0 {
		t.Fatalf("links/sync requests without a runtime manager = %d, want 0", got)
	}
}

// TestValidateExternalLinkValueIsThePanelsOwnBar: the node accepts pushed rows
// without a session, so the value check here is the only one that runs for them.
func TestValidateExternalLinkValueIsThePanelsOwnBar(t *testing.T) {
	cases := []struct {
		name  string
		kind  string
		value string
		want  string
	}{
		{"share link", model.ExternalLinkKindLink, "trojan://user@example.com:443", ""},
		{"share link that does not parse", model.ExternalLinkKindLink, "just-some-text", "unsupported or invalid share link: just-some-text"},
		{"https subscription", model.ExternalLinkKindSubscription, "https://provider.example/sub", ""},
		{"http subscription", model.ExternalLinkKindSubscription, "http://provider.example/sub", ""},
		{"subscription over ftp", model.ExternalLinkKindSubscription, "ftp://provider.example/sub", "external subscription must be an http(s) URL: ftp://provider.example/sub"},
		{"subscription without a scheme", model.ExternalLinkKindSubscription, "provider.example/sub", "external subscription must be an http(s) URL: provider.example/sub"},
		{"unknown kind", "bogus", "trojan://user@example.com:443", "unknown external link kind: bogus"},
		{"empty value", model.ExternalLinkKindLink, "", "pushed link is missing its kind or value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExternalLinkValue(tc.kind, tc.value)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("validate(%q, %q) = %v, want nil", tc.kind, tc.value, err)
				}
				return
			}
			if err == nil || strings.TrimSpace(err.Error()) != tc.want {
				t.Fatalf("validate(%q, %q) = %v, want %q", tc.kind, tc.value, err, tc.want)
			}
		})
	}
}
