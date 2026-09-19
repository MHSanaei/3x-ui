package service

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// EffectiveExternalLink is one library link as a single client receives it: the
// scope that granted it plus that scope's overrides over the shared row.
type EffectiveExternalLink struct {
	LinkId      int
	Kind        string
	Value       string
	Remark      string
	NamePrefix  string
	Enable      bool
	ExpiryTime  int64
	SortIndex   int
	Scope       string
	ScopeTarget int
	Origin      string
	UserAgent   string
	Headers     map[string]string
	CacheTTL    int
}

// externalLinkScopeRank decides which assignment wins when a client receives
// the same link from several scopes: the most specific one does.
var externalLinkScopeRank = map[string]int{
	model.ExternalLinkTargetClient:  3,
	model.ExternalLinkTargetInbound: 2,
	model.ExternalLinkTargetGroup:   1,
	model.ExternalLinkTargetGlobal:  0,
}

// ResolveEffectiveExternalLinks returns the links each client of clientIds
// receives, already ordered. One lookup per table, never one per client.
func ResolveEffectiveExternalLinks(clientIds []int) (map[int][]EffectiveExternalLink, error) {
	resolved := make(map[int][]EffectiveExternalLink, len(clientIds))
	ids := uniqueInts(clientIds)
	if len(ids) == 0 {
		return resolved, nil
	}
	db := database.GetDB()

	clients := make([]model.ClientRecord, 0, len(ids))
	if err := db.Select("id", "group_name").Where("id IN ?", ids).Find(&clients).Error; err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return resolved, nil
	}

	inboundsByClient := map[int][]int{}
	inboundIds := []int{}
	for _, batch := range chunkInts(ids, sqlInChunk) {
		var rows []model.ClientInbound
		if err := db.Select("client_id", "inbound_id").Where("client_id IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			inboundsByClient[row.ClientId] = append(inboundsByClient[row.ClientId], row.InboundId)
			inboundIds = append(inboundIds, row.InboundId)
		}
	}

	groupIdsByClient, groupIds, err := resolveClientGroupIds(db, clients)
	if err != nil {
		return nil, err
	}

	byScope, err := loadExternalLinkAssignmentScopes(db, ids, uniqueInts(inboundIds), groupIds)
	if err != nil {
		return nil, err
	}
	library, err := loadExternalLinksByIds(db, assignmentLinkIds(byScope))
	if err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	for _, client := range clients {
		candidates := byScope[externalLinkScopeKey(model.ExternalLinkTargetGlobal, 0)]
		candidates = append(candidates, byScope[externalLinkScopeKey(model.ExternalLinkTargetClient, client.Id)]...)
		if groupId, ok := groupIdsByClient[client.Id]; ok {
			candidates = append(candidates, byScope[externalLinkScopeKey(model.ExternalLinkTargetGroup, groupId)]...)
		}
		for _, inboundId := range inboundsByClient[client.Id] {
			candidates = append(candidates, byScope[externalLinkScopeKey(model.ExternalLinkTargetInbound, inboundId)]...)
		}
		resolved[client.Id] = effectiveLinksForClient(candidates, library, now)
	}
	return resolved, nil
}

// resolveClientGroupIds maps each client to the id of its group. Clients may
// carry a group name with no stored row, and assignments are keyed by id.
func resolveClientGroupIds(db *gorm.DB, clients []model.ClientRecord) (map[int]int, []int, error) {
	byClient := map[int]int{}
	names := make([]string, 0, len(clients))
	seen := map[string]struct{}{}
	for _, c := range clients {
		name := strings.TrimSpace(c.Group)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 {
		return byClient, nil, nil
	}
	idByName := map[string]int{}
	groupIds := []int{}
	for _, batch := range chunkStrings(names, sqlInChunk) {
		var groups []model.ClientGroup
		if err := db.Select("id", "name").Where("name IN ?", batch).Find(&groups).Error; err != nil {
			return nil, nil, err
		}
		for _, g := range groups {
			idByName[g.Name] = g.Id
			groupIds = append(groupIds, g.Id)
		}
	}
	for _, c := range clients {
		if id, ok := idByName[strings.TrimSpace(c.Group)]; ok {
			byClient[c.Id] = id
		}
	}
	return byClient, groupIds, nil
}

func externalLinkScopeKey(targetType string, targetId int) string {
	return targetType + "\x00" + strconv.Itoa(targetId)
}

// loadExternalLinkAssignmentScopes collects every assignment that could reach
// one of the given clients, grouped by scope target for the per-client pass.
func loadExternalLinkAssignmentScopes(db *gorm.DB, clientIds, inboundIds, groupIds []int) (map[string][]model.ExternalLinkAssignment, error) {
	byScope := map[string][]model.ExternalLinkAssignment{}
	collect := func(targetType string, ids []int, chunked bool) error {
		query := func(batch []int) error {
			var rows []model.ExternalLinkAssignment
			where := "target_type = ?"
			args := []any{targetType}
			if len(batch) > 0 {
				where += " AND target_id IN ?"
				args = append(args, batch)
			}
			if err := db.Where(where, args...).Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				key := externalLinkScopeKey(row.TargetType, row.TargetId)
				byScope[key] = append(byScope[key], row)
			}
			return nil
		}
		if !chunked {
			return query(nil)
		}
		for _, batch := range chunkInts(ids, sqlInChunk) {
			if err := query(batch); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collect(model.ExternalLinkTargetGlobal, nil, false); err != nil {
		return nil, err
	}
	if err := collect(model.ExternalLinkTargetClient, clientIds, true); err != nil {
		return nil, err
	}
	if len(groupIds) > 0 {
		if err := collect(model.ExternalLinkTargetGroup, groupIds, true); err != nil {
			return nil, err
		}
	}
	if len(inboundIds) > 0 {
		if err := collect(model.ExternalLinkTargetInbound, inboundIds, true); err != nil {
			return nil, err
		}
	}
	return byScope, nil
}

func assignmentLinkIds(byScope map[string][]model.ExternalLinkAssignment) []int {
	seen := map[int]struct{}{}
	ids := []int{}
	for _, rows := range byScope {
		for _, row := range rows {
			if _, ok := seen[row.LinkId]; ok {
				continue
			}
			seen[row.LinkId] = struct{}{}
			ids = append(ids, row.LinkId)
		}
	}
	return ids
}

func loadExternalLinksByIds(db *gorm.DB, ids []int) (map[int]model.ExternalLink, error) {
	library := make(map[int]model.ExternalLink, len(ids))
	for _, batch := range chunkInts(ids, sqlInChunk) {
		var rows []model.ExternalLink
		if err := db.Where("id IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			library[row.Id] = row
		}
	}
	return library, nil
}

// effectiveLinksForClient flattens the candidate assignments of one client into
// the ordered set it receives, dropping disabled and expired links.
func effectiveLinksForClient(candidates []model.ExternalLinkAssignment, library map[int]model.ExternalLink, now int64) []EffectiveExternalLink {
	best := make(map[int]model.ExternalLinkAssignment, len(candidates))
	for _, candidate := range candidates {
		current, ok := best[candidate.LinkId]
		if ok && externalLinkScopeRank[current.TargetType] >= externalLinkScopeRank[candidate.TargetType] {
			continue
		}
		best[candidate.LinkId] = candidate
	}

	out := make([]EffectiveExternalLink, 0, len(best))
	for linkId, assignment := range best {
		link, ok := library[linkId]
		if !ok {
			continue
		}
		enable := link.Enable == nil || *link.Enable
		if assignment.Enable != nil {
			enable = *assignment.Enable
		}
		if !enable {
			continue
		}
		expiry := resolvedExpiry(link.ExpiryTime, assignment.ExpiryTime)
		if expiry > 0 && expiry <= now {
			continue
		}
		remark := link.Remark
		if assignment.Remark != "" {
			remark = assignment.Remark
		}
		prefix := link.NamePrefix
		if assignment.NamePrefix != "" {
			prefix = assignment.NamePrefix
		}
		sortIndex := link.SortIndex
		if assignment.TargetType == model.ExternalLinkTargetClient {
			sortIndex = assignment.SortIndex
		}
		out = append(out, EffectiveExternalLink{
			LinkId:      linkId,
			Kind:        link.Kind,
			Value:       link.Value,
			Remark:      remark,
			NamePrefix:  prefix,
			Enable:      enable,
			ExpiryTime:  expiry,
			SortIndex:   sortIndex,
			Scope:       assignment.TargetType,
			ScopeTarget: assignment.TargetId,
			Origin:      assignment.Origin,
			UserAgent:   link.UserAgent,
			Headers:     link.Headers,
			CacheTTL:    link.CacheTTL,
		})
	}
	sortExternalLinks(out)
	return out
}

// sortExternalLinks keeps a client's own rows first, in the order the operator
// gave them, then the inherited ones in library order.
func sortExternalLinks(links []EffectiveExternalLink) {
	sort.SliceStable(links, func(i, j int) bool {
		directI := links[i].Scope == model.ExternalLinkTargetClient
		directJ := links[j].Scope == model.ExternalLinkTargetClient
		if directI != directJ {
			return directI
		}
		if links[i].SortIndex != links[j].SortIndex {
			return links[i].SortIndex < links[j].SortIndex
		}
		return links[i].LinkId < links[j].LinkId
	})
}
