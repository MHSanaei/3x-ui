package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// servedLink is the part of a resolved link a subscription actually renders, so
// two resolutions compare equal regardless of the scope that granted them.
// Ordering is compared by position instead: sort_index is an ordering key whose
// value legitimately differs between a library row and a client binding.
type servedLink struct {
	Kind       string
	Value      string
	Remark     string
	NamePrefix string
	Enable     bool
	ExpiryTime int64
	UserAgent  string
	CacheTTL   int
}

func servedOf(links []EffectiveExternalLink) []servedLink {
	out := make([]servedLink, 0, len(links))
	for _, link := range links {
		out = append(out, servedLink{
			Kind:       link.Kind,
			Value:      link.Value,
			Remark:     link.Remark,
			NamePrefix: link.NamePrefix,
			Enable:     link.Enable,
			ExpiryTime: link.ExpiryTime,
			UserAgent:  link.UserAgent,
			CacheTTL:   link.CacheTTL,
		})
	}
	return out
}

// nodeInboundFor hands one seeded inbound to a node, which is what puts its
// clients into that node's snapshot.
func nodeInboundFor(t *testing.T, inboundId, nodeID int) {
	t.Helper()
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", inboundId).
		Update("node_id", nodeID).Error; err != nil {
		t.Fatalf("attach inbound %d to node %d: %v", inboundId, nodeID, err)
	}
}

func nodeSyncPayload(t *testing.T, nodeID int) *runtime.ExternalLinkSync {
	t.Helper()
	payload, err := (&ClientService{}).ExternalLinkSyncForNode(nodeID)
	if err != nil {
		t.Fatalf("ExternalLinkSyncForNode(%d): %v", nodeID, err)
	}
	return payload
}

// TestExternalLinkSyncReproducesTheMastersResolutionOnANode is the property this
// protocol exists for: a node that holds none of the panel's scopes must still
// serve exactly the links the master resolved for that client.
func TestExternalLinkSyncReproducesTheMastersResolutionOnANode(t *testing.T) {
	nodeInbound, ids := seedLibraryInbound(t, "node-in-1", 21101, []model.Client{
		libraryClient("node-a@x", "paid"),
		libraryClient("node-b@x", "paid"),
	})
	// A client of the panel's own inbound: the push must leave it out, because
	// the node does not serve it.
	localClient := libraryClient("local-c@x", "paid")
	seedInboundConflict(t, "local-in-1", "0.0.0.0", 21102, model.VLESS,
		`{"network":"tcp"}`, clientsSettings(t, []model.Client{localClient}))
	localInbound := loadInboundByTag(t, "local-in-1")
	if err := (&ClientService{}).SyncInbound(nil, localInbound.Id, []model.Client{localClient}); err != nil {
		t.Fatalf("SyncInbound(local-in-1): %v", err)
	}
	localId := lookupClientRecord(t, "local-c@x").Id
	nodeInboundFor(t, nodeInbound.Id, 7)
	seedLibraryGroup(t, "paid")

	global := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://glob", "global")
	group := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://grp", "group")
	disabled := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://off", "disabled")
	own := seedLibraryLink(t, model.ExternalLinkKindSubscription, "https://provider.example/sub", "provider")
	// The shared row carries an expiry while the client's own binding says never:
	// the push has to keep that "never" instead of letting the node inherit it.
	if err := database.GetDB().Model(&model.ExternalLink{}).Where("id = ?", own.Id).
		Update("expiry_time", time.Now().Add(time.Hour).UnixMilli()).Error; err != nil {
		t.Fatalf("set shared expiry: %v", err)
	}
	assignLibraryLink(t, global.Id, model.ExternalLinkTargetGlobal, 0, "")
	assignLibraryLink(t, group.Id, model.ExternalLinkTargetGroup, groupRowId(t, "paid"), "")
	assignLibraryLink(t, disabled.Id, model.ExternalLinkTargetGlobal, 0, "")
	assignLibraryLink(t, own.Id, model.ExternalLinkTargetClient, ids["node-a@x"], "own provider")
	// The panel's own binding for a client stores the "never" sentinel, because 0
	// there means "inherit the shared row".
	if err := database.GetDB().Model(&model.ExternalLinkAssignment{}).
		Where("link_id = ? AND target_type = ? AND target_id = ?", own.Id,
			model.ExternalLinkTargetClient, ids["node-a@x"]).
		Update("expiry_time", model.ExternalLinkExpiryNever).Error; err != nil {
		t.Fatalf("mark own binding as never expiring: %v", err)
	}

	if err := (&ClientService{}).ExternalLinkLibrarySetEnable(disabled.Id, false); err != nil {
		t.Fatalf("disable link: %v", err)
	}

	// The master's own view, captured before the store is replaced by the push.
	want := servedOf(resolveOneClient(t, ids["node-a@x"]))
	if len(want) != 3 {
		t.Fatalf("master resolved %d links, want 3: %+v", len(want), want)
	}
	if want[0].ExpiryTime != 0 {
		t.Fatalf("the client's own link resolved expiry %d, want 0 (never)", want[0].ExpiryTime)
	}

	payload := nodeSyncPayload(t, 7)
	if len(payload.Clients) != 2 {
		served := []string{}
		for _, c := range payload.Clients {
			served = append(served, c.Email)
		}
		t.Fatalf("payload clients = %v, want only the node's two", served)
	}
	for _, link := range payload.Links {
		if link.Value == "trojan://off" {
			t.Error("payload carries the disabled link")
		}
	}

	// A node holds none of the master's scopes: wipe the store and let the push
	// rebuild it, which is the state a fresh node starts from.
	if err := database.GetDB().Exec("DELETE FROM external_link_assignments").Error; err != nil {
		t.Fatalf("clear assignments: %v", err)
	}
	if err := database.GetDB().Exec("DELETE FROM external_links").Error; err != nil {
		t.Fatalf("clear library: %v", err)
	}
	if err := (&ClientService{}).ApplyExternalLinkSync(payload); err != nil {
		t.Fatalf("apply push: %v", err)
	}

	got := servedOf(resolveOneClient(t, ids["node-a@x"]))
	if len(got) != len(want) {
		t.Fatalf("node resolved %d links, master served %d: %+v vs %+v", len(got), len(want), got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("link %d = %+v, want %+v", index, got[index], want[index])
		}
	}

	// The node's other client keeps its own resolution, and the client that does
	// not live on this node is untouched by the push.
	if local := servedOf(resolveOneClient(t, localId)); len(local) != 0 {
		t.Errorf("local client resolved %d links, want none: %+v", len(local), local)
	}
}

// TestAppliedPushedRowsAreReadOnlyOnTheNode pins the guard a master's authority
// depends on: without it the operator's edit here is silently undone by the
// next push.
func TestAppliedPushedRowsAreReadOnlyOnTheNode(t *testing.T) {
	nodeInbound, ids := seedLibraryInbound(t, "node-in-2", 21103, []model.Client{
		libraryClient("node-d@x", ""),
	})
	nodeInboundFor(t, nodeInbound.Id, 9)
	link := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://pushed", "pushed")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetClient, ids["node-d@x"], "")

	payload := nodeSyncPayload(t, 9)
	if err := (&ClientService{}).ApplyExternalLinkSync(payload); err != nil {
		t.Fatalf("apply push: %v", err)
	}

	pushed := libraryRowByValue(t, "trojan://pushed")
	if pushed.Origin != model.ExternalLinkOriginNode {
		t.Fatalf("pushed row origin = %q, want %q", pushed.Origin, model.ExternalLinkOriginNode)
	}

	edit := pushed
	edit.Remark = "renamed here"
	if err := (&ClientService{}).ExternalLinkLibrarySave(&edit); err == nil {
		t.Error("editing a master-pushed row was allowed")
	}
	if err := (&ClientService{}).ExternalLinkLibrarySetEnable(pushed.Id, false); err == nil {
		t.Error("disabling a master-pushed row was allowed")
	}
	if err := (&ClientService{}).ExternalLinkLibraryDelete(pushed.Id); err == nil {
		t.Error("deleting a master-pushed row was allowed")
	}

	panel := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://own-row", "own")
	panel.Remark = "renamed here"
	if err := (&ClientService{}).ExternalLinkLibrarySave(&panel); err != nil {
		t.Errorf("editing the panel's own row failed: %v", err)
	}
}

// TestApplyExternalLinkSyncKeepsPanelRowsAndDropsStalePushes pins the other half
// of the guard: a push replacing its own bindings must not take the rows this
// panel owns with it.
func TestApplyExternalLinkSyncKeepsPanelRowsAndDropsStalePushes(t *testing.T) {
	nodeInbound, ids := seedLibraryInbound(t, "node-in-3", 21104, []model.Client{
		libraryClient("node-e@x", ""),
	})
	nodeInboundFor(t, nodeInbound.Id, 11)
	clientId := ids["node-e@x"]

	granted := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://granted", "granted")
	assignLibraryLink(t, granted.Id, model.ExternalLinkTargetClient, clientId, "")
	payload := nodeSyncPayload(t, 11)

	// Two rows the push knows nothing about, both seeded after the snapshot: one
	// an earlier push left behind, one this panel added for its own client.
	stale := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://stale", "stale")
	assignLibraryLink(t, stale.Id, model.ExternalLinkTargetClient, clientId, "")
	if err := database.GetDB().Model(&model.ExternalLinkAssignment{}).
		Where("link_id = ? AND target_id = ?", stale.Id, clientId).
		Update("origin", model.ExternalLinkOriginNode).Error; err != nil {
		t.Fatalf("mark stale binding as pushed: %v", err)
	}
	if err := database.GetDB().Model(&model.ExternalLink{}).Where("id = ?", stale.Id).
		Update("origin", model.ExternalLinkOriginNode).Error; err != nil {
		t.Fatalf("mark stale library row as pushed: %v", err)
	}
	mine := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://mine", "mine")
	assignLibraryLink(t, mine.Id, model.ExternalLinkTargetClient, clientId, "")

	if err := (&ClientService{}).ApplyExternalLinkSync(payload); err != nil {
		t.Fatalf("apply push: %v", err)
	}

	assertAssignmentCount(t, granted.Id, model.ExternalLinkTargetClient, clientId, 1)
	assertAssignmentCount(t, stale.Id, model.ExternalLinkTargetClient, clientId, 0)
	assertLibraryRowSurvives(t, mine.Id)
	assertAssignmentCount(t, mine.Id, model.ExternalLinkTargetClient, clientId, 1)

	// Applying the same payload twice must not duplicate anything.
	countBefore := countRows(t, &model.ExternalLinkAssignment{})
	if err := (&ClientService{}).ApplyExternalLinkSync(payload); err != nil {
		t.Fatalf("re-apply push: %v", err)
	}
	if countAfter := countRows(t, &model.ExternalLinkAssignment{}); countAfter != countBefore {
		t.Errorf("assignments after re-apply = %d, want %d", countAfter, countBefore)
	}
	if rows := countRows(t, &model.ExternalLink{}); rows != 2 {
		t.Errorf("library rows = %d, want 2: the stale pushed row is retired, the panel's own stays", rows)
	}
}

// TestApplyExternalLinkSyncIgnoresAnEmptySnapshot pins the safety property that
// makes an empty push harmless: a node that momentarily reports no inbounds
// must not lose the links it was already given.
func TestApplyExternalLinkSyncIgnoresAnEmptySnapshot(t *testing.T) {
	nodeInbound, ids := seedLibraryInbound(t, "node-in-4", 21105, []model.Client{
		libraryClient("node-f@x", ""),
	})
	nodeInboundFor(t, nodeInbound.Id, 13)
	link := seedLibraryLink(t, model.ExternalLinkKindLink, "trojan://kept", "kept")
	assignLibraryLink(t, link.Id, model.ExternalLinkTargetClient, ids["node-f@x"], "")

	payload := nodeSyncPayload(t, 13)
	if err := (&ClientService{}).ApplyExternalLinkSync(payload); err != nil {
		t.Fatalf("apply push: %v", err)
	}
	before := countRows(t, &model.ExternalLinkAssignment{})

	if err := (&ClientService{}).ApplyExternalLinkSync(&runtime.ExternalLinkSync{}); err != nil {
		t.Fatalf("apply empty push: %v", err)
	}
	if after := countRows(t, &model.ExternalLinkAssignment{}); after != before {
		t.Errorf("assignments after an empty push = %d, want %d", after, before)
	}
	if rows := countRows(t, &model.ExternalLink{}); rows != 1 {
		t.Errorf("library rows after an empty push = %d, want 1", rows)
	}
}

func countRows(t *testing.T, mdl any) int64 {
	t.Helper()
	var count int64
	if err := database.GetDB().Model(mdl).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", mdl, err)
	}
	return count
}

func libraryRowByValue(t *testing.T, value string) model.ExternalLink {
	t.Helper()
	var row model.ExternalLink
	if err := database.GetDB().Where("value = ?", value).First(&row).Error; err != nil {
		t.Fatalf("load library row %q: %v", value, err)
	}
	return row
}

func groupRowId(t *testing.T, name string) int {
	t.Helper()
	var row model.ClientGroup
	if err := database.GetDB().Where("name = ?", name).First(&row).Error; err != nil {
		t.Fatalf("load group %q: %v", name, err)
	}
	return row.Id
}
