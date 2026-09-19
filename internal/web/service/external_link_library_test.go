package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// seedLibraryLink creates one shared library row and returns it. The library is
// deduped by (kind, value), so the same value always yields the same row.
func seedLibraryLink(t *testing.T, kind, value, remark string) model.ExternalLink {
	t.Helper()
	link := model.ExternalLink{Kind: kind, Value: value, Remark: remark}
	if err := database.GetDB().Create(&link).Error; err != nil {
		t.Fatalf("create library link %q: %v", value, err)
	}
	return link
}

// assignLibraryLink binds one library row to a target, optionally overriding
// the naming the library row carries.
func assignLibraryLink(t *testing.T, linkId int, targetType string, targetId int, remark string) {
	t.Helper()
	row := model.ExternalLinkAssignment{
		LinkId:     linkId,
		TargetType: targetType,
		TargetId:   targetId,
		Remark:     remark,
		Origin:     model.ExternalLinkOriginPanel,
	}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatalf("assign link %d to %s/%d: %v", linkId, targetType, targetId, err)
	}
}

// seedLibraryInbound seeds one inbound holding the given clients and returns it
// with the emails' record ids.
func seedLibraryInbound(t *testing.T, tag string, port int, clients []model.Client) (*model.Inbound, map[string]int) {
	t.Helper()
	setupConflictDB(t)
	seedInboundConflict(t, tag, "0.0.0.0", port, model.VLESS, `{"network":"tcp"}`, clientsSettings(t, clients))
	inbound := loadInboundByTag(t, tag)
	if err := (&ClientService{}).SyncInbound(nil, inbound.Id, clients); err != nil {
		t.Fatalf("SyncInbound(%s): %v", tag, err)
	}
	ids := map[string]int{}
	for _, client := range clients {
		rec := lookupClientRecord(t, client.Email)
		ids[client.Email] = rec.Id
	}
	return inbound, ids
}

// seedLibraryGroup creates the stored group row a group-scoped assignment is
// keyed by. Clients carry the group name; the row is what gives it an id.
func seedLibraryGroup(t *testing.T, name string) model.ClientGroup {
	t.Helper()
	row := model.ClientGroup{Name: name}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatalf("create group %q: %v", name, err)
	}
	return row
}

func libraryClient(email, group string) model.Client {
	return model.Client{Email: email, Group: group, ID: "1f8d5e1c-3f3b-4d2a-9f7e-4b1c2d3e4f50", Enable: true}
}

// TestEffectiveExternalLinksScopePrecedence pins the resolution rule: own beats
// inbound, inbound beats group, group beats the panel-wide assignment.
func TestEffectiveExternalLinksScopePrecedence(t *testing.T) {
	const (
		tag   = "awg-lib-priority"
		email = "priority@example.test"
		group = "priority-group"
	)
	inbound, ids := seedLibraryInbound(t, tag, 26110, []model.Client{libraryClient(email, group)})

	link := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@priority.example:443", "library")
	groupRow := seedLibraryGroup(t, group)
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetGlobal, 0, "global")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetGroup, groupRow.Id, "group")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetInbound, inbound.Id, "inbound")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetClient, ids[email], "own")

	cases := []struct {
		name   string
		remove func()
		want   string
	}{
		{"own beats every inherited scope", func() {}, "own"},
		{"inbound beats group and global", func() {
			deleteLibraryAssignment(t, link.Id, model.ExternalLinkTargetClient, ids[email])
		}, "inbound"},
		{"group beats global", func() {
			deleteLibraryAssignment(t, link.Id, model.ExternalLinkTargetInbound, inbound.Id)
		}, "group"},
		{"global is the fallback", func() {
			deleteLibraryAssignment(t, link.Id, model.ExternalLinkTargetGroup, groupRow.Id)
		}, "global"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.remove()
			links := resolveOneClient(t, ids[email])
			if len(links) != 1 {
				t.Fatalf("resolved %d link(s), want 1: %+v", len(links), links)
			}
			if links[0].Remark != tc.want {
				t.Fatalf("remark = %q, want %q", links[0].Remark, tc.want)
			}
		})
	}
}

// TestEffectiveExternalLinksSkipDisabledAndExpired keeps a row that is off out
// of the emitted set at both levels, and drops an expired one.
func TestEffectiveExternalLinksSkipDisabledAndExpired(t *testing.T) {
	const email = "off@example.test"
	_, ids := seedLibraryInbound(t, "awg-lib-off", 26111, []model.Client{libraryClient(email, "")})

	live := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@live.example:443", "live")
	assignLibraryLink(t, live.Id, model.ExternalLinkTargetGlobal, 0, "")

	offAtTarget := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@off-target.example:443", "off-target")
	assignLibraryLink(t, offAtTarget.Id, model.ExternalLinkTargetGlobal, 0, "")
	if err := database.GetDB().Model(&model.ExternalLinkAssignment{}).
		Where("link_id = ? AND target_type = ?", offAtTarget.Id, model.ExternalLinkTargetGlobal).
		Update("enable", false).Error; err != nil {
		t.Fatalf("disable assignment: %v", err)
	}

	expired := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@expired.example:443", "expired")
	assignLibraryLink(t, expired.Id, model.ExternalLinkTargetGlobal, 0, "")
	if err := database.GetDB().Model(&model.ExternalLink{}).Where("id = ?", expired.Id).
		Update("expiry_time", int64(1)).Error; err != nil {
		t.Fatalf("expire library row: %v", err)
	}

	links := resolveOneClient(t, ids[email])
	if len(links) != 1 || links[0].Remark != "live" {
		t.Fatalf("resolved = %+v, want only the live link", links)
	}
}

// TestNewClientDefaultsApplyToNewClientsOnly: the default reaches newly created
// clients only, and is copied onto them so it stays editable per client.
func TestNewClientDefaultsApplyToNewClientsOnly(t *testing.T) {
	const (
		older = "older@example.test"
		newer = "newer@example.test"
	)
	_, ids := seedLibraryInbound(t, "awg-lib-default", 26112, []model.Client{libraryClient(older, "")})

	link := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@default.example:443", "default")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetNewClients, 0, "")

	if links := resolveOneClient(t, ids[older]); len(links) != 0 {
		t.Fatalf("pre-existing client received the new-clients default: %+v", links)
	}

	inbound := loadInboundByTag(t, "awg-lib-default")
	client := libraryClient(newer, "")
	client.ID = "2f8d5e1c-3f3b-4d2a-9f7e-4b1c2d3e4f51"
	if err := (&ClientService{}).SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound(new client): %v", err)
	}
	newId := lookupClientRecord(t, newer).Id
	links := resolveOneClient(t, newId)
	if len(links) != 1 || links[0].Remark != "default" {
		t.Fatalf("new client resolved = %+v, want the default link", links)
	}
	var own int64
	if err := database.GetDB().Model(&model.ExternalLinkAssignment{}).
		Where("link_id = ? AND target_type = ? AND target_id = ?", link.Id, model.ExternalLinkTargetClient, newId).
		Count(&own).Error; err != nil {
		t.Fatalf("count materialized assignment: %v", err)
	}
	if own != 1 {
		t.Fatalf("materialized client assignments = %d, want 1", own)
	}
}

// TestExternalLinkCascadesDropAssignmentsNotLibraryRows: deleting the client,
// inbound or group drops that scope's assignments and keeps the shared row.
func TestExternalLinkCascadesDropAssignmentsNotLibraryRows(t *testing.T) {
	const (
		email = "cascade@example.test"
		group = "cascade-group"
		tag   = "awg-lib-cascade"
	)
	inbound, ids := seedLibraryInbound(t, tag, 26113, []model.Client{libraryClient(email, group)})
	clientId := ids[email]

	groupRow := seedLibraryGroup(t, group)

	inboundLink := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@inbound-scope.example:443", "inbound-scope")
	assignLibraryLink(t, inboundLink.Id, model.ExternalLinkTargetInbound, inbound.Id, "")
	groupLink := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@group-scope.example:443", "group-scope")
	assignLibraryLink(t, groupLink.Id, model.ExternalLinkTargetGroup, groupRow.Id, "")

	if _, err := (&InboundService{}).DelInbound(inbound.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	assertAssignmentCount(t, inboundLink.Id, model.ExternalLinkTargetInbound, inbound.Id, 0)
	assertLibraryRowSurvives(t, inboundLink.Id)

	if _, err := (&ClientService{}).DeleteGroup(group); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	assertAssignmentCount(t, groupLink.Id, model.ExternalLinkTargetGroup, groupRow.Id, 0)
	assertLibraryRowSurvives(t, groupLink.Id)

	ownLink := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@own-scope.example:443", "own-scope")
	assignLibraryLink(t, ownLink.Id, model.ExternalLinkTargetClient, clientId, "")
	if _, err := (&ClientService{}).Delete(&InboundService{}, clientId, false); err != nil {
		t.Fatalf("Delete(client): %v", err)
	}
	assertAssignmentCount(t, ownLink.Id, model.ExternalLinkTargetClient, clientId, 0)
	assertLibraryRowSurvives(t, ownLink.Id)
}

// TestExportImportRoundTripsOwnExternalLinks keeps a client's own links and their
// overrides across an export and a re-import, without duplicating shared rows.
func TestExportImportRoundTripsOwnExternalLinks(t *testing.T) {
	const email = "portable-links@example.test"
	setupConflictDB(t)
	inbound := mkInbound(t, 26114, model.VLESS, `{"clients":[]}`)

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	client := libraryClient(email, "")
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{Client: client, InboundIds: []int{inbound.Id}}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	clientId := lookupClientRecord(t, email).Id

	own := []ExternalLinkInput{
		{Kind: model.ExternalLinkKindLink, Value: "vless://uuid@ported.example:443", Remark: "ported", NamePrefix: "[p] "},
		{Kind: model.ExternalLinkKindSubscription, Value: "https://provider.example/sub/ported", Remark: "provider"},
	}
	if err := svc.SetExternalLinksForRecord(clientId, own); err != nil {
		t.Fatalf("SetExternalLinksForRecord: %v", err)
	}

	exported, err := svc.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if len(exported) != 1 || len(exported[0].ExternalLinks) != 2 {
		t.Fatalf("export carried %d entries with %d link(s), want 1 with 2", len(exported), len(exported[0].ExternalLinks))
	}
	if _, err := svc.Delete(inboundSvc, clientId, false); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	assertAssignmentCount(t, 0, model.ExternalLinkTargetClient, clientId, 0)

	result, _, err := svc.ImportClients(inboundSvc, exported)
	if err != nil {
		t.Fatalf("ImportClients: %v", err)
	}
	if result.Created != 1 {
		t.Fatalf("import result = %+v", result)
	}
	restoredId := lookupClientRecord(t, email).Id
	restored := resolveOneClient(t, restoredId)
	if len(restored) != 2 {
		t.Fatalf("restored %d link(s), want 2: %+v", len(restored), restored)
	}
	byRemark := map[string]EffectiveExternalLink{}
	for _, link := range restored {
		byRemark[link.Remark] = link
	}
	if got := byRemark["ported"]; got.Value != own[0].Value || got.NamePrefix != "[p] " {
		t.Fatalf("restored own link = %+v, want value %q with prefix %q", got, own[0].Value, "[p] ")
	}
	if _, ok := byRemark["provider"]; !ok {
		t.Fatalf("subscription row was lost on import: %+v", restored)
	}
	var rows int64
	if err := database.GetDB().Model(&model.ExternalLink{}).
		Where("kind = ? AND value = ?", model.ExternalLinkKindLink, own[0].Value).Count(&rows).Error; err != nil {
		t.Fatalf("count library rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("library rows for one value = %d, want 1 (import must reuse, not duplicate)", rows)
	}
}

// TestLibraryUsageCountsDistinctClientsAcrossScopes pins the count the library
// page shows: clients reached through any scope, each counted once.
func TestLibraryUsageCountsDistinctClientsAcrossScopes(t *testing.T) {
	const (
		one   = "usage-one@example.test"
		two   = "usage-two@example.test"
		group = "usage-group"
	)
	_, ids := seedLibraryInbound(t, "awg-lib-usage", 26115, []model.Client{
		libraryClient(one, group),
		libraryClient(two, ""),
	})

	link := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@usage.example:443", "usage")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetGlobal, 0, "")
	groupRow := seedLibraryGroup(t, group)
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetGroup, groupRow.Id, "")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetClient, ids[one], "")

	rows, err := (&ClientService{}).ExternalLinkLibraryList()
	if err != nil {
		t.Fatalf("ExternalLinkLibraryList: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("library rows = %d, want 1", len(rows))
	}
	if rows[0].AssignedClients != 2 {
		t.Fatalf("assignedClients = %d, want 2 distinct clients", rows[0].AssignedClients)
	}
}

// TestLibrarySaveKeepsOrderAndIdentityOnPartialEdit: an edit that carries only
// the renamed remark must not reorder the row or erase the fetch identity it
// needs to revalidate its subscription.
func TestLibrarySaveKeepsOrderAndIdentityOnPartialEdit(t *testing.T) {
	setupConflictDB(t)
	service := &ClientService{}

	target := model.ExternalLink{
		Kind:      model.ExternalLinkKindSubscription,
		Value:     "https://partial.example/sub",
		Remark:    "before",
		UserAgent: "Happ/2.5.1",
		CacheTTL:  600,
	}
	if err := service.ExternalLinkLibrarySave(&target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	others := []*model.ExternalLink{
		{Kind: model.ExternalLinkKindLink, Value: "vless://uuid@one.example:443#one", Remark: "one"},
		{Kind: model.ExternalLinkKindLink, Value: "vless://uuid@two.example:443#two", Remark: "two"},
	}
	for _, other := range others {
		if err := service.ExternalLinkLibrarySave(other); err != nil {
			t.Fatalf("create %s: %v", other.Remark, err)
		}
	}
	if err := service.ExternalLinkLibraryReorder([]int{others[0].Id, others[1].Id, target.Id}); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	if err := service.ExternalLinkLibrarySetEnable(target.Id, false); err != nil {
		t.Fatalf("disable target: %v", err)
	}

	partial := model.ExternalLink{Id: target.Id, Kind: target.Kind, Value: target.Value, Remark: "after"}
	if err := service.ExternalLinkLibrarySave(&partial); err != nil {
		t.Fatalf("partial edit: %v", err)
	}

	got, err := service.ExternalLinkLibraryGet(target.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Remark != "after" {
		t.Fatalf("remark = %q, want the edit to land", got.Remark)
	}
	if got.Enable == nil || *got.Enable {
		t.Fatalf("enable = %v, want the row to stay disabled: the edit did not mention it", got.Enable)
	}
	if got.SortIndex != 2 {
		t.Fatalf("sortIndex = %d, want the reorder's 2 kept: only reorder owns the order", got.SortIndex)
	}
	if got.UserAgent != "Happ/2.5.1" || got.CacheTTL != 600 {
		t.Fatalf("userAgent/cacheTtl = %q/%d, want the row's fetch identity kept", got.UserAgent, got.CacheTTL)
	}
}

// TestLibrarySaveAppliesTheFetchIdentityItIsGiven is the other half of the
// partial edit: the columns a request does carry must land, or editing a
// subscription's User-Agent, headers or cache TTL from the library page would
// silently keep the old ones.
func TestLibrarySaveAppliesTheFetchIdentityItIsGiven(t *testing.T) {
	setupConflictDB(t)
	svc := &ClientService{}

	row := &model.ExternalLink{
		Kind:      model.ExternalLinkKindSubscription,
		Value:     "https://identity.example/sub",
		Remark:    "before",
		UserAgent: "Happ/2.5.1",
		Headers:   map[string]string{"X-Device-Model": "Pixel 9"},
		CacheTTL:  600,
	}
	if err := svc.ExternalLinkLibrarySave(row); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	edit := &model.ExternalLink{
		Id:         row.Id,
		Kind:       model.ExternalLinkKindSubscription,
		Value:      row.Value,
		Remark:     "after",
		NamePrefix: "[zjh] ",
		Enable:     boolPtr(true),
		ExpiryTime: 1893456000000,
		UserAgent:  "Happ/2.6.0",
		Headers:    map[string]string{"X-Device-Model": "iPhone 17"},
		CacheTTL:   1200,
	}
	if err := svc.ExternalLinkLibrarySave(edit); err != nil {
		t.Fatalf("edit subscription: %v", err)
	}

	got, err := svc.ExternalLinkLibraryGet(row.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UserAgent != "Happ/2.6.0" || got.Headers["X-Device-Model"] != "iPhone 17" || got.CacheTTL != 1200 {
		t.Fatalf("fetch identity = %q %v %d, want the edit's values", got.UserAgent, got.Headers, got.CacheTTL)
	}
	if got.Remark != "after" || got.NamePrefix != "[zjh] " || got.ExpiryTime != 1893456000000 {
		t.Fatalf("edited row = %+v, want the request's remark, prefix and expiry", got)
	}
	if got.Enable == nil || !*got.Enable {
		t.Fatalf("enable = %v, want the request's true", got.Enable)
	}
}

// TestLibrarySaveRefusesToMergeTwoRowsIdentity: renaming one row onto another
// row's (kind, value) has to be refused with a message the operator can act on,
// not by letting the unique index fail the write.
func TestLibrarySaveRefusesToMergeTwoRowsIdentity(t *testing.T) {
	setupConflictDB(t)
	svc := &ClientService{}

	first := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://identity-taken", "first")
	second := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://identity-mine", "second")

	err := svc.ExternalLinkLibrarySave(&model.ExternalLink{
		Id: second.Id, Kind: model.ExternalLinkKindLink, Value: first.Value, Remark: "renamed",
	})
	if err == nil || strings.TrimSpace(err.Error()) != "this link is already in the library: "+first.Value {
		t.Fatalf("err = %v, want the duplicate-identity refusal", err)
	}
	got, err := svc.ExternalLinkLibraryGet(second.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Value != "trojan://identity-mine" || got.Remark != "second" {
		t.Fatalf("refused edit changed the row: %+v", got)
	}
}

// TestLibraryUsageCountsEachRowOnItsOwn: two shared rows with different reach
// must report their own count. Without a GROUP BY the aggregate collapses into
// one row carrying the panel total, which a single-row seed cannot see.
func TestLibraryUsageCountsEachRowOnItsOwn(t *testing.T) {
	const owner = "usage-one@example.test"
	_, ids := seedLibraryInbound(t, "awg-lib-usage", 26112, []model.Client{
		libraryClient(owner, ""),
		libraryClient("usage-two@example.test", ""),
	})

	wide := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@wide.example:443", "wide")
	assignLibraryLink(t, wide.Id, model.ExternalLinkTargetGlobal, 0, "")

	narrow := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@narrow.example:443", "narrow")
	assignLibraryLink(t, narrow.Id, model.ExternalLinkTargetClient, ids[owner], "")

	rows, err := (&ClientService{}).ExternalLinkLibraryList()
	if err != nil {
		t.Fatalf("ExternalLinkLibraryList: %v", err)
	}
	got := map[string]int{}
	for _, row := range rows {
		got[row.Remark] = row.AssignedClients
	}
	if got["wide"] != 2 || got["narrow"] != 1 {
		t.Fatalf("assignedClients = %v, want wide=2 and narrow=1: one row must not carry another's clients", got)
	}
}

// TestAssignmentExpiryNeverBeatsTheSharedRow: a client-owned 0 means "never",
// so a shorter expiry on the shared row must not cut that client off.
func TestAssignmentExpiryNeverBeatsTheSharedRow(t *testing.T) {
	const (
		direct  = "never-direct@example.test"
		inherit = "never-inherit@example.test"
	)
	_, ids := seedLibraryInbound(t, "awg-lib-never", 26113, []model.Client{
		libraryClient(direct, ""),
		libraryClient(inherit, ""),
	})

	shared := seedLibraryLink(t, model.ExternalLinkKindLink, "vless://uuid@never.example:443", "never")
	if err := database.GetDB().Model(&model.ExternalLink{}).Where("id = ?", shared.Id).
		Update("expiry_time", int64(1)).Error; err != nil {
		t.Fatalf("expire the shared row: %v", err)
	}
	assignLibraryLink(t, shared.Id, model.ExternalLinkTargetClient, ids[direct], "")
	if err := database.GetDB().Model(&model.ExternalLinkAssignment{}).
		Where("link_id = ? AND target_type = ? AND target_id = ?", shared.Id, model.ExternalLinkTargetClient, ids[direct]).
		Update("expiry_time", model.ExternalLinkExpiryNever).Error; err != nil {
		t.Fatalf("pin the assignment as never-expiring: %v", err)
	}

	if links := resolveOneClient(t, ids[direct]); len(links) != 1 {
		t.Fatalf("resolved for the owning client = %+v, want the link kept: its own expiry says never", links)
	}
	assignLibraryLink(t, shared.Id, model.ExternalLinkTargetClient, ids[inherit], "")
	if links := resolveOneClient(t, ids[inherit]); len(links) != 0 {
		t.Fatalf("resolved for an inheriting client = %+v, want none: the shared row is expired", links)
	}
}

func resolveOneClient(t *testing.T, clientId int) []EffectiveExternalLink {
	t.Helper()
	resolved, err := ResolveEffectiveExternalLinks([]int{clientId})
	if err != nil {
		t.Fatalf("ResolveEffectiveExternalLinks(%d): %v", clientId, err)
	}
	return resolved[clientId]
}

func deleteLibraryAssignment(t *testing.T, linkId int, targetType string, targetId int) {
	t.Helper()
	if err := database.GetDB().Where("link_id = ? AND target_type = ? AND target_id = ?",
		linkId, targetType, targetId).Delete(&model.ExternalLinkAssignment{}).Error; err != nil {
		t.Fatalf("delete assignment: %v", err)
	}
}

func assertAssignmentCount(t *testing.T, linkId int, targetType string, targetId int, want int64) {
	t.Helper()
	query := database.GetDB().Model(&model.ExternalLinkAssignment{}).
		Where("target_type = ? AND target_id = ?", targetType, targetId)
	if linkId > 0 {
		query = query.Where("link_id = ?", linkId)
	}
	var got int64
	if err := query.Count(&got).Error; err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if got != want {
		t.Fatalf("assignments for %s/%d = %d, want %d", targetType, targetId, got, want)
	}
}

func assertLibraryRowSurvives(t *testing.T, linkId int) {
	t.Helper()
	var got int64
	if err := database.GetDB().Model(&model.ExternalLink{}).Where("id = ?", linkId).Count(&got).Error; err != nil {
		t.Fatalf("count library row: %v", err)
	}
	if got != 1 {
		t.Fatalf("library row %d was deleted with its last assignment; want it kept for reuse", linkId)
	}
}
