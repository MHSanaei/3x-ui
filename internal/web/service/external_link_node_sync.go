package service

import (
	"context"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// nodeClientRef is one client an inbound of a node carries: the master knows it
// by id, the node by email, and a push has to speak both.
type nodeClientRef struct {
	Id    int
	Email string
}

// clientsOfNode lists the clients attached to the inbounds one node owns. A
// client spanning two nodes appears for both, which is what each one serves.
func clientsOfNode(db *gorm.DB, nodeID int) ([]nodeClientRef, error) {
	refs := []nodeClientRef{}
	if err := db.Table("client_inbounds").
		Select("DISTINCT clients.id AS id, clients.email AS email").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Joins("JOIN inbounds ON inbounds.id = client_inbounds.inbound_id").
		Where("inbounds.node_id = ?", nodeID).
		Scan(&refs).Error; err != nil {
		return nil, err
	}
	return refs, nil
}

// ExternalLinkSyncForNode resolves what every client of one node must receive.
// Group and inbound scopes are flattened here on purpose: the node stores no
// groups, so a scope it cannot evaluate has to arrive as a client binding.
func (s *ClientService) ExternalLinkSyncForNode(nodeID int) (*runtime.ExternalLinkSync, error) {
	return s.externalLinkSyncForClients(nodeID, nil)
}

// externalLinkSyncForClients restricts the snapshot to the given clients when
// ids is non-empty, so one new client costs a few rows instead of the node's
// whole set. An empty ids list means the node's complete set.
func (s *ClientService) externalLinkSyncForClients(nodeID int, ids []int) (*runtime.ExternalLinkSync, error) {
	if nodeID <= 0 {
		return nil, common.NewError("node id must be positive")
	}
	db := database.GetDB()
	refs, err := clientsOfNode(db, nodeID)
	if err != nil {
		return nil, err
	}
	selected := map[int]struct{}{}
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	sync := &runtime.ExternalLinkSync{
		Links:   []runtime.ExternalLinkSyncLink{},
		Clients: []runtime.ExternalLinkSyncClient{},
	}
	if len(refs) == 0 {
		return sync, nil
	}

	clientIds := make([]int, 0, len(refs))
	for _, ref := range refs {
		if len(selected) > 0 {
			if _, ok := selected[ref.Id]; !ok {
				continue
			}
		}
		clientIds = append(clientIds, ref.Id)
	}
	if len(clientIds) == 0 {
		return sync, nil
	}

	resolved, err := ResolveEffectiveExternalLinks(clientIds)
	if err != nil {
		return nil, err
	}
	linkIds := []int{}
	for _, links := range resolved {
		for _, link := range links {
			linkIds = append(linkIds, link.LinkId)
		}
	}
	library, err := loadExternalLinksByIds(db, uniqueInts(linkIds))
	if err != nil {
		return nil, err
	}

	emitted := map[int]struct{}{}
	for _, ref := range refs {
		links, ok := resolved[ref.Id]
		if !ok {
			continue
		}
		client := runtime.ExternalLinkSyncClient{
			Email: ref.Email,
			Links: make([]runtime.ExternalLinkSyncAssignment, 0, len(links)),
		}
		for index, link := range links {
			row, found := library[link.LinkId]
			if !found {
				continue
			}
			if _, done := emitted[row.Id]; !done {
				emitted[row.Id] = struct{}{}
				sync.Links = append(sync.Links, runtime.ExternalLinkSyncLink{
					Kind:       row.Kind,
					Value:      row.Value,
					Remark:     row.Remark,
					NamePrefix: row.NamePrefix,
					Enable:     row.Enable == nil || *row.Enable,
					ExpiryTime: row.ExpiryTime,
					SortIndex:  row.SortIndex,
					UserAgent:  row.UserAgent,
					Headers:    row.Headers,
					CacheTtl:   row.CacheTTL,
				})
			}
			client.Links = append(client.Links, runtime.ExternalLinkSyncAssignment{
				Kind:       row.Kind,
				Value:      row.Value,
				Enable:     link.Enable,
				ExpiryTime: link.ExpiryTime,
				Remark:     link.Remark,
				NamePrefix: link.NamePrefix,
				// The node re-sorts client bindings by this, so it has to be the
				// position in the order the master would have served.
				SortIndex: index,
			})
		}
		sync.Clients = append(sync.Clients, client)
	}
	return sync, nil
}

// PushExternalLinksToNode hands one node the links its clients inherit. A
// failure is logged, never returned: the next edit or reconcile re-pushes.
func (s *ClientService) PushExternalLinksToNode(n *model.Node) {
	if n == nil || n.Id <= 0 || !n.Enable {
		return
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return
	}
	payload, err := s.ExternalLinkSyncForNode(n.Id)
	if err != nil {
		logger.Warningf("external link sync: snapshot for node %s failed: %v", n.Name, err)
		return
	}
	s.pushExternalLinkSync(mgr, n, payload)
}

// PushExternalLinksToNodes fans out to every enabled node, a few at a time so
// one hanging node cannot outlast an operator's request.
func (s *ClientService) PushExternalLinksToNodes() {
	mgr := runtime.GetManager()
	if mgr == nil {
		return
	}
	nodes, err := (&NodeService{}).GetAll()
	if err != nil {
		logger.Warningf("external link sync: node list failed: %v", err)
		return
	}
	var wg sync.WaitGroup
	gate := make(chan struct{}, nodeFanoutConcurrency)
	for _, n := range nodes {
		if n == nil || !n.Enable {
			continue
		}
		payload, err := s.ExternalLinkSyncForNode(n.Id)
		if err != nil {
			logger.Warningf("external link sync: snapshot for node %s failed: %v", n.Name, err)
			continue
		}
		wg.Add(1)
		gate <- struct{}{}
		go func(node *model.Node, snapshot *runtime.ExternalLinkSync) {
			defer wg.Done()
			defer func() { <-gate }()
			s.pushExternalLinkSync(mgr, node, snapshot)
		}(n, payload)
	}
	wg.Wait()
}

// PushExternalLinksForEmails re-pushes just the clients one operation touched,
// which is what a single client add or edit costs: the node's other clients are
// already converged, and a bulk operation defers to the reconcile sweep instead.
func (s *ClientService) PushExternalLinksForEmails(nodeID int, emails []string) {
	if nodeID <= 0 || len(emails) == 0 {
		return
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return
	}
	node, err := (&NodeService{}).GetById(nodeID)
	if err != nil || node == nil || !node.Enable {
		return
	}
	ids, err := clientIdsByEmail(database.GetDB(), emails)
	if err != nil {
		logger.Warningf("external link sync: client lookup for node %s failed: %v", node.Name, err)
		return
	}
	clientIds := make([]int, 0, len(ids))
	for _, id := range ids {
		clientIds = append(clientIds, id)
	}
	payload, err := s.externalLinkSyncForClients(nodeID, clientIds)
	if err != nil {
		logger.Warningf("external link sync: snapshot for node %s failed: %v", node.Name, err)
		return
	}
	s.pushExternalLinkSync(mgr, node, payload)
}

func (s *ClientService) pushExternalLinkSync(mgr *runtime.Manager, n *model.Node, payload *runtime.ExternalLinkSync) {
	// An offline node is not reachable now: its next reconcile pushes the set,
	// and waiting here would only make the operator's edit slow.
	if n.Status != "online" {
		return
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		logger.Warningf("external link sync: remote lookup failed for %s: %v", n.Name, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), nodeClientPushTimeout)
	defer cancel()
	if err := remote.PushExternalLinks(ctx, *payload); err != nil {
		logger.Warningf("external link sync: push to %s failed: %v", n.Name, err)
	}
}

// ApplyExternalLinkSync materializes a master's snapshot in this panel's store.
// Pushed rows carry origin "node", which is what keeps them read-only here and
// what a local save leaves alone; the master's copy always stays authoritative.
func (s *ClientService) ApplyExternalLinkSync(payload *runtime.ExternalLinkSync) error {
	if payload == nil {
		return common.NewError("sync payload is required")
	}
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := upsertPushedExternalLinks(tx, payload.Links); err != nil {
			return err
		}
		emails := make([]string, 0, len(payload.Clients))
		for _, client := range payload.Clients {
			if email := strings.TrimSpace(client.Email); email != "" {
				emails = append(emails, email)
			}
		}
		clientIds, err := clientIdsByEmail(tx, emails)
		if err != nil {
			return err
		}
		for _, client := range payload.Clients {
			id, ok := clientIds[strings.TrimSpace(client.Email)]
			if !ok {
				// The node does not hold this client (yet): a later push lands
				// when the client itself has been pushed.
				continue
			}
			if err := replacePushedClientLinks(tx, id, client.Links); err != nil {
				return err
			}
		}
		return dropUnusedPushedExternalLinks(tx)
	})
}

func upsertPushedExternalLinks(tx *gorm.DB, links []runtime.ExternalLinkSyncLink) error {
	for _, entry := range links {
		kind := strings.TrimSpace(entry.Kind)
		value := strings.TrimSpace(entry.Value)
		// Same bar as the panel's own save: a push must not be able to store a
		// value this panel would have refused, whoever signs the request.
		if err := validateExternalLinkValue(kind, value); err != nil {
			return err
		}
		enable := entry.Enable
		row := model.ExternalLink{
			Kind:       kind,
			Value:      value,
			Remark:     entry.Remark,
			NamePrefix: entry.NamePrefix,
			Enable:     &enable,
			ExpiryTime: entry.ExpiryTime,
			SortIndex:  entry.SortIndex,
			UserAgent:  entry.UserAgent,
			Headers:    entry.Headers,
			CacheTTL:   entry.CacheTtl,
			Origin:     model.ExternalLinkOriginNode,
		}
		// An existing row with this identity is taken over, not duplicated: the
		// unique index makes it the same link, and the master owns its fetch
		// fields from here on.
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "kind"}, {Name: "value"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"remark", "name_prefix", "enable", "expiry_time", "sort_index",
				"user_agent", "headers", "cache_ttl", "origin",
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// validateExternalLinkValue is the check the library's own save applies, shared
// here so a pushed row cannot be anything the panel would have rejected.
func validateExternalLinkValue(kind, value string) error {
	if value == "" {
		return common.NewError("pushed link is missing its kind or value")
	}
	switch kind {
	case model.ExternalLinkKindLink:
		if _, err := link.ParseLink(value); err != nil {
			return common.NewError("unsupported or invalid share link: " + value)
		}
	case model.ExternalLinkKindSubscription:
		if !isHTTPURL(value) {
			return common.NewError("external subscription must be an http(s) URL: " + value)
		}
	default:
		return common.NewError("unknown external link kind: " + kind)
	}
	return nil
}

func clientIdsByEmail(tx *gorm.DB, emails []string) (map[string]int, error) {
	out := map[string]int{}
	unique := []string{}
	seen := map[string]struct{}{}
	for _, email := range emails {
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		unique = append(unique, email)
	}
	for _, batch := range chunkStrings(unique, sqlInChunk) {
		var rows []model.ClientRecord
		if err := tx.Select("id", "email").Where("email IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.Email] = row.Id
		}
	}
	return out, nil
}

// replacePushedClientLinks swaps the pushed bindings of one client for the ones
// the master just resolved, leaving the client's own panel rows untouched.
func replacePushedClientLinks(tx *gorm.DB, clientId int, links []runtime.ExternalLinkSyncAssignment) error {
	if err := tx.Where("target_type = ? AND target_id = ? AND origin = ?",
		model.ExternalLinkTargetClient, clientId, model.ExternalLinkOriginNode).
		Delete(&model.ExternalLinkAssignment{}).Error; err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}
	idsByKey, err := externalLinkIdsByIdentity(tx, links)
	if err != nil {
		return err
	}
	rows := make([]model.ExternalLinkAssignment, 0, len(links))
	for _, entry := range links {
		linkId, ok := idsByKey[model.ExternalLinkIdentity(entry.Kind, entry.Value)]
		if !ok {
			continue
		}
		enable := entry.Enable
		rows = append(rows, model.ExternalLinkAssignment{
			LinkId:     linkId,
			TargetType: model.ExternalLinkTargetClient,
			TargetId:   clientId,
			Enable:     &enable,
			// The master sent a resolved expiry where 0 means never, which is
			// the sentinel here: 0 on this column would inherit the shared row.
			ExpiryTime: expiryForPushedAssignment(entry.ExpiryTime),
			Remark:     entry.Remark,
			NamePrefix: entry.NamePrefix,
			SortIndex:  entry.SortIndex,
			Origin:     model.ExternalLinkOriginNode,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	// The client may already hold this link as its own binding here, and the
	// index allows one row per (link, target): the pushed copy takes it over,
	// or a local override would quietly contradict the master.
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "link_id"}, {Name: "target_type"}, {Name: "target_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enable", "expiry_time", "remark", "name_prefix", "sort_index", "origin",
		}),
	}).Create(&rows).Error
}

func expiryForPushedAssignment(expiryTime int64) int64 {
	if expiryTime <= 0 {
		return model.ExternalLinkExpiryNever
	}
	return expiryTime
}

func externalLinkIdsByIdentity(tx *gorm.DB, links []runtime.ExternalLinkSyncAssignment) (map[string]int, error) {
	out := map[string]int{}
	values := []string{}
	seen := map[string]struct{}{}
	for _, entry := range links {
		value := strings.TrimSpace(entry.Value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	for _, batch := range chunkStrings(values, sqlInChunk) {
		var rows []model.ExternalLink
		if err := tx.Select("id", "kind", "value").Where("value IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[model.ExternalLinkIdentity(row.Kind, row.Value)] = row.Id
		}
	}
	return out, nil
}

// dropUnusedPushedExternalLinks retires library rows the master no longer hands
// out, so a node's library does not accumulate links nobody inherits.
func dropUnusedPushedExternalLinks(tx *gorm.DB) error {
	return tx.Where("origin = ? AND id NOT IN (?)", model.ExternalLinkOriginNode,
		tx.Model(&model.ExternalLinkAssignment{}).Select("link_id")).
		Delete(&model.ExternalLink{}).Error
}
