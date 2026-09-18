package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExternalLinkView is one row of a client's Links tab: the library entry, the
// overrides of the scope that granted it, and the fetch status to show.
type ExternalLinkView struct {
	AssignmentId   int    `json:"assignmentId" example:"4"`
	LinkId         int    `json:"linkId" example:"1"`
	Kind           string `json:"kind" example:"link"`
	Value          string `json:"value" example:"vless://uuid@example.com:443#node"`
	Remark         string `json:"remark" example:"Backup provider"`
	NamePrefix     string `json:"namePrefix" example:""`
	Enable         bool   `json:"enable" example:"true"`
	ExpiryTime     int64  `json:"expiryTime" example:"0"`
	Scope          string `json:"scope" example:"client"`
	ScopeTarget    int    `json:"scopeTarget" example:"7"`
	Own            bool   `json:"own" example:"true"`
	LastFetchAt    int64  `json:"lastFetchAt" example:"0"`
	LastFetchError string `json:"lastFetchError" example:""`
	UserAgent      string `json:"userAgent" example:""`
	CacheTtl       int    `json:"cacheTtl" example:"0"`
}

// ExternalLinkAssignRequest names the targets of an assign or unassign call.
// More than one target may be given in a single call.
type ExternalLinkAssignRequest struct {
	Emails     []string `json:"emails" form:"emails"`
	Group      string   `json:"group" form:"group"`
	InboundId  int      `json:"inboundId" form:"inboundId"`
	Global     bool     `json:"global" form:"global"`
	NewClients bool     `json:"newClients" form:"newClients"`
	Enable     *bool    `json:"enable" form:"enable"`
	ExpiryTime int64    `json:"expiryTime" form:"expiryTime"`
	NamePrefix string   `json:"namePrefix" form:"namePrefix"`
}

// ExternalLinkTargetView is one target a library entry is assigned to.
type ExternalLinkTargetView struct {
	TargetType string `json:"targetType" example:"client"`
	TargetId   int    `json:"targetId" example:"7"`
	Name       string `json:"name" example:"user@example.com"`
}

// ExternalLinkLibraryList returns the library in display order, each row
// carrying how many clients it reaches.
func (s *ClientService) ExternalLinkLibraryList() ([]model.ExternalLink, error) {
	db := database.GetDB()
	links := []model.ExternalLink{}
	if err := db.Order("sort_index ASC, id ASC").Find(&links).Error; err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return links, nil
	}
	usage, err := externalLinkClientUsage(db)
	if err != nil {
		return nil, err
	}
	for i := range links {
		links[i].AssignedClients = usage[links[i].Id]
	}
	return links, nil
}

func (s *ClientService) ExternalLinkLibraryGet(id int) (*model.ExternalLink, error) {
	var link model.ExternalLink
	if err := database.GetDB().Where("id = ?", id).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

// ExternalLinkLibrarySave creates the entry, or updates it in place when it
// carries an id. Every client inheriting the row sees the edit immediately.
func (s *ClientService) ExternalLinkLibrarySave(link *model.ExternalLink) error {
	if link == nil {
		return common.NewError("link is required")
	}
	if link.Id > 0 {
		if err := ensureLinkEditable(database.GetDB(), link.Id); err != nil {
			return err
		}
	}
	rows, err := normalizeExternalLinks([]ExternalLinkInput{{
		Kind:       link.Kind,
		Value:      link.Value,
		Remark:     link.Remark,
		Enable:     link.Enable,
		ExpiryTime: link.ExpiryTime,
		NamePrefix: link.NamePrefix,
	}})
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return common.NewError("link value is required")
	}
	link.Kind = rows[0].Kind
	link.Value = rows[0].Value
	link.Remark = rows[0].Remark
	link.NamePrefix = rows[0].NamePrefix
	link.ExpiryTime = rows[0].ExpiryTime
	// A body that omits enable must leave the row's state alone: only a request
	// that carries the field may flip it.
	enableProvided := link.Enable != nil
	if link.Enable == nil {
		link.Enable = rows[0].Enable
	}

	db := database.GetDB()
	var duplicate int64
	if err := db.Model(&model.ExternalLink{}).
		Where("kind = ? AND value = ? AND id <> ?", link.Kind, link.Value, link.Id).
		Count(&duplicate).Error; err != nil {
		return err
	}
	if duplicate > 0 {
		return common.NewError("this link is already in the library: " + link.Value)
	}
	if link.Id > 0 {
		// A partial edit must not reorder the row or erase its fetch identity:
		// reorder owns sort_index, and the identity columns change only when sent.
		fields := map[string]any{
			"kind":        link.Kind,
			"value":       link.Value,
			"remark":      link.Remark,
			"name_prefix": link.NamePrefix,
			"expiry_time": link.ExpiryTime,
		}
		if enableProvided {
			fields["enable"] = link.Enable
		}
		if link.UserAgent != "" {
			fields["user_agent"] = link.UserAgent
		}
		if len(link.Headers) > 0 {
			fields["headers"] = link.Headers
		}
		if link.CacheTTL > 0 {
			fields["cache_ttl"] = link.CacheTTL
		}
		// The marker rides the write's own transaction: a crash between the two
		// would leave every node serving the library this edit replaced.
		return db.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&model.ExternalLink{}).Where("id = ?", link.Id).Updates(fields)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return common.NewError("link not found")
			}
			return (&NodeService{}).MarkAllNodesDirtyTx(tx)
		})
	}
	var maxIndex int
	if err := db.Model(&model.ExternalLink{}).Select("COALESCE(MAX(sort_index), -1)").Scan(&maxIndex).Error; err != nil {
		return err
	}
	link.SortIndex = maxIndex + 1
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(link).Error; err != nil {
			return err
		}
		return (&NodeService{}).MarkAllNodesDirtyTx(tx)
	})
}

// ensureLinkEditable refuses to touch a row a master pushed: the next sync would
// overwrite the change, so the operator has to edit it where it came from.
func ensureLinkEditable(db *gorm.DB, id int) error {
	var row model.ExternalLink
	err := db.Select("id", "origin", "value").Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.NewError("link not found")
	}
	if err != nil {
		return err
	}
	if row.Origin == model.ExternalLinkOriginNode {
		return common.NewError("this link is managed by the master panel: " + row.Value)
	}
	return nil
}

func (s *ClientService) ExternalLinkLibraryDelete(id int) error {
	if err := ensureLinkEditable(database.GetDB(), id); err != nil {
		return err
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("link_id = ?", id).Delete(&model.ExternalLinkAssignment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", id).Delete(&model.ExternalLink{}).Error; err != nil {
			return err
		}
		return (&NodeService{}).MarkAllNodesDirtyTx(tx)
	})
}

func (s *ClientService) ExternalLinkLibrarySetEnable(id int, enable bool) error {
	if err := ensureLinkEditable(database.GetDB(), id); err != nil {
		return err
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ExternalLink{}).Where("id = ?", id).
			UpdateColumn("enable", enable)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return common.NewError("link not found")
		}
		return (&NodeService{}).MarkAllNodesDirtyTx(tx)
	})
}

// ExternalLinkLibraryReorder stores the given ids in the order they arrive.
func (s *ClientService) ExternalLinkLibraryReorder(ids []int) error {
	ids = uniqueInts(ids)
	if len(ids) == 0 {
		return common.NewError("ids are required")
	}
	for _, id := range ids {
		if err := ensureLinkEditable(database.GetDB(), id); err != nil {
			return err
		}
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		for index, id := range ids {
			if err := tx.Model(&model.ExternalLink{}).Where("id = ?", id).
				UpdateColumn("sort_index", index).Error; err != nil {
				return err
			}
		}
		return (&NodeService{}).MarkAllNodesDirtyTx(tx)
	})
}

// ExternalLinkAssign binds a library entry to every named target and returns
// how many assignments were created or refreshed.
func (s *ClientService) ExternalLinkAssign(linkId int, req ExternalLinkAssignRequest) (int, error) {
	db := database.GetDB()
	if _, err := s.ExternalLinkLibraryGet(linkId); err != nil {
		return 0, common.NewError("link not found")
	}
	targets, err := resolveAssignTargets(db, req, true)
	if err != nil {
		return 0, err
	}
	affected := 0
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, target := range targets {
			assignment := model.ExternalLinkAssignment{
				LinkId:     linkId,
				TargetType: target.TargetType,
				TargetId:   target.TargetId,
				Enable:     req.Enable,
				ExpiryTime: req.ExpiryTime,
				NamePrefix: req.NamePrefix,
				Origin:     model.ExternalLinkOriginPanel,
			}
			res := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "link_id"}, {Name: "target_type"}, {Name: "target_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"enable", "expiry_time", "name_prefix"}),
			}).Create(&assignment)
			if res.Error != nil {
				return res.Error
			}
			affected++
		}
		return (&NodeService{}).MarkAllNodesDirtyTx(tx)
	})
	return affected, err
}

// ExternalLinkUnassign removes the named targets and returns how many
// assignments were deleted.
func (s *ClientService) ExternalLinkUnassign(linkId int, req ExternalLinkAssignRequest) (int, error) {
	db := database.GetDB()
	targets, err := resolveAssignTargets(db, req, false)
	if err != nil {
		return 0, err
	}
	deleted := 0
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, target := range targets {
			res := tx.Where("link_id = ? AND target_type = ? AND target_id = ?",
				linkId, target.TargetType, target.TargetId).Delete(&model.ExternalLinkAssignment{})
			if res.Error != nil {
				return res.Error
			}
			deleted += int(res.RowsAffected)
		}
		return (&NodeService{}).MarkAllNodesDirtyTx(tx)
	})
	return deleted, err
}

// ExternalLinkTargets lists what a library entry is assigned to, for the page.
func (s *ClientService) ExternalLinkTargets(linkId int) ([]ExternalLinkTargetView, error) {
	db := database.GetDB()
	var assignments []model.ExternalLinkAssignment
	if err := db.Where("link_id = ?", linkId).Order("target_type ASC, target_id ASC").
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	out := make([]ExternalLinkTargetView, 0, len(assignments))
	emails := map[int]string{}
	groups := map[int]string{}
	inbounds := map[int]string{}
	for _, a := range assignments {
		switch a.TargetType {
		case model.ExternalLinkTargetClient:
			emails[a.TargetId] = ""
		case model.ExternalLinkTargetGroup:
			groups[a.TargetId] = ""
		case model.ExternalLinkTargetInbound:
			inbounds[a.TargetId] = ""
		}
	}
	if err := fillNamesById(db, &model.ClientRecord{}, "email", emails); err != nil {
		return nil, err
	}
	if err := fillNamesById(db, &model.ClientGroup{}, "name", groups); err != nil {
		return nil, err
	}
	if err := fillNamesById(db, &model.Inbound{}, "tag", inbounds); err != nil {
		return nil, err
	}
	for _, a := range assignments {
		view := ExternalLinkTargetView{TargetType: a.TargetType, TargetId: a.TargetId}
		switch a.TargetType {
		case model.ExternalLinkTargetClient:
			view.Name = emails[a.TargetId]
		case model.ExternalLinkTargetGroup:
			view.Name = groups[a.TargetId]
		case model.ExternalLinkTargetInbound:
			view.Name = inbounds[a.TargetId]
		case model.ExternalLinkTargetGlobal:
			view.Name = "all clients"
		case model.ExternalLinkTargetNewClients:
			view.Name = "new clients"
		}
		out = append(out, view)
	}
	return out, nil
}

// ClientExternalLinkViews returns the links one client receives: its own first,
// then the inherited ones with the scope that granted them.
func (s *ClientService) ClientExternalLinkViews(clientId int) ([]ExternalLinkView, error) {
	resolved, err := ResolveEffectiveExternalLinks([]int{clientId})
	if err != nil {
		return nil, err
	}
	db := database.GetDB()
	assignments := map[int]int{}
	var own []model.ExternalLinkAssignment
	if err := db.Where("link_id IN ? AND target_type = ? AND target_id = ?",
		externalLinkIdsOf(resolved[clientId]), model.ExternalLinkTargetClient, clientId).
		Find(&own).Error; err != nil {
		return nil, err
	}
	for _, a := range own {
		assignments[a.LinkId] = a.Id
	}
	status, err := externalLinkFetchStatus(db, externalLinkIdsOf(resolved[clientId]))
	if err != nil {
		return nil, err
	}
	out := make([]ExternalLinkView, 0, len(resolved[clientId]))
	for _, link := range resolved[clientId] {
		view := ExternalLinkView{
			AssignmentId:   assignments[link.LinkId],
			LinkId:         link.LinkId,
			Kind:           link.Kind,
			Value:          link.Value,
			Remark:         link.Remark,
			NamePrefix:     link.NamePrefix,
			Enable:         link.Enable,
			ExpiryTime:     link.ExpiryTime,
			Scope:          link.Scope,
			ScopeTarget:    link.ScopeTarget,
			Own:            link.Scope == model.ExternalLinkTargetClient,
			LastFetchAt:    status[link.LinkId].LastFetchAt,
			LastFetchError: status[link.LinkId].LastFetchError,
			UserAgent:      link.UserAgent,
			CacheTtl:       link.CacheTTL,
		}
		out = append(out, view)
	}
	return out, nil
}

// RecordExternalLinkFetchByValue stamps the fetch outcome on every library row
// holding this URL. A 304 passes nil links, keeping the last good expansion.
func RecordExternalLinkFetchByValue(value string, links []string, fetchErr error) error {
	value = strings.TrimSpace(value)
	db := database.GetDB()
	if value == "" || db == nil {
		return nil
	}
	updates := map[string]any{
		"last_fetch_at":    time.Now().UnixMilli(),
		"last_fetch_error": "",
	}
	if fetchErr != nil {
		updates["last_fetch_error"] = fetchErr.Error()
	} else if links != nil {
		// A map update bypasses the field serializer, so encode by hand.
		encoded, err := json.Marshal(links)
		if err != nil {
			return err
		}
		updates["last_links"] = string(encoded)
	}
	return db.Model(&model.ExternalLink{}).
		Where("kind = ? AND value = ?", model.ExternalLinkKindSubscription, value).
		Updates(updates).Error
}

// LastExternalLinkLinksByValue is the last good expansion stored for one
// subscription URL: a restart with the provider down still serves the client.
func LastExternalLinkLinksByValue(value string) []string {
	value = strings.TrimSpace(value)
	db := database.GetDB()
	if value == "" || db == nil {
		return nil
	}
	var row model.ExternalLink
	if err := db.Select("last_links").
		Where("kind = ? AND value = ?", model.ExternalLinkKindSubscription, value).
		First(&row).Error; err != nil {
		return nil
	}
	return row.LastLinks
}

// dropExternalLinkAssignmentsTx removes the assignments a deleted row owned: a
// client going away must not take a link other clients still inherit.
func dropExternalLinkAssignmentsTx(tx *gorm.DB, targetType string, ids ...int) error {
	for _, batch := range chunkInts(ids, sqlInChunk) {
		if err := tx.Where("target_type = ? AND target_id IN ?", targetType, batch).
			Delete(&model.ExternalLinkAssignment{}).Error; err != nil {
			return err
		}
	}
	return nil
}

// materializeNewClientExternalLinks copies the "new clients" defaults onto
// freshly created clients: one read and one batch insert for any client count.
func materializeNewClientExternalLinks(tx *gorm.DB, clientIds ...int) error {
	if len(clientIds) == 0 {
		return nil
	}
	var defaults []model.ExternalLinkAssignment
	if err := tx.Where("target_type = ?", model.ExternalLinkTargetNewClients).
		Order("link_id ASC").Find(&defaults).Error; err != nil {
		return err
	}
	if len(defaults) == 0 {
		return nil
	}
	rows := make([]model.ExternalLinkAssignment, 0, len(defaults)*len(clientIds))
	for _, clientId := range clientIds {
		for _, def := range defaults {
			rows = append(rows, model.ExternalLinkAssignment{
				LinkId:     def.LinkId,
				TargetType: model.ExternalLinkTargetClient,
				TargetId:   clientId,
				Enable:     def.Enable,
				ExpiryTime: def.ExpiryTime,
				Remark:     def.Remark,
				NamePrefix: def.NamePrefix,
				SortIndex:  def.SortIndex,
				Origin:     model.ExternalLinkOriginPanel,
			})
		}
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 200).Error
}

// externalLinkClientUsage counts distinct clients per library entry across all
// scopes in one query, so the library page needs no per-row resolution.
func externalLinkClientUsage(db *gorm.DB) (map[int]int, error) {
	var rows []struct {
		LinkId  int
		Clients int
	}
	query := `
		SELECT link_id, COUNT(DISTINCT client_id) AS clients FROM (
			SELECT a.link_id AS link_id, a.target_id AS client_id
				FROM external_link_assignments a WHERE a.target_type = 'client'
			UNION ALL
			SELECT a.link_id, c.id
				FROM external_link_assignments a
				JOIN client_groups g ON g.id = a.target_id
				JOIN clients c ON c.group_name = g.name
				WHERE a.target_type = 'group'
			UNION ALL
			SELECT a.link_id, ci.client_id
				FROM external_link_assignments a
				JOIN client_inbounds ci ON ci.inbound_id = a.target_id
				WHERE a.target_type = 'inbound'
			UNION ALL
			SELECT a.link_id, c.id
				FROM external_link_assignments a
				JOIN clients c ON 1 = 1
				WHERE a.target_type = 'global'
		) AS reach
		GROUP BY link_id`
	if err := db.Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}
	usage := make(map[int]int, len(rows))
	for _, row := range rows {
		usage[row.LinkId] = row.Clients
	}
	return usage, nil
}

func externalLinkFetchStatus(db *gorm.DB, ids []int) (map[int]model.ExternalLink, error) {
	out := map[int]model.ExternalLink{}
	for _, batch := range chunkInts(ids, sqlInChunk) {
		var rows []model.ExternalLink
		if err := db.Select("id", "last_fetch_at", "last_fetch_error").
			Where("id IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.Id] = row
		}
	}
	return out, nil
}

func externalLinkIdsOf(links []EffectiveExternalLink) []int {
	out := make([]int, 0, len(links))
	for _, link := range links {
		out = append(out, link.LinkId)
	}
	return out
}

func fillNamesById(db *gorm.DB, dest any, column string, ids map[int]string) error {
	if len(ids) == 0 {
		return nil
	}
	list := make([]int, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	type named struct {
		Id   int
		Name string
	}
	for _, batch := range chunkInts(list, sqlInChunk) {
		var rows []named
		if err := db.Model(dest).Select("id", column+" AS name").Where("id IN ?", batch).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			ids[row.Id] = row.Name
		}
	}
	return nil
}

// resolveAssignTargets turns a request's target selectors into concrete
// (type, id) pairs. createGroups creates a typed group name, as bulkAdd does.
func resolveAssignTargets(db *gorm.DB, req ExternalLinkAssignRequest, createGroups bool) ([]ExternalLinkTargetView, error) {
	targets := []ExternalLinkTargetView{}
	if req.Global {
		targets = append(targets, ExternalLinkTargetView{TargetType: model.ExternalLinkTargetGlobal})
	}
	if req.NewClients {
		targets = append(targets, ExternalLinkTargetView{TargetType: model.ExternalLinkTargetNewClients})
	}
	emails := []string{}
	for _, email := range req.Emails {
		if trimmed := strings.TrimSpace(email); trimmed != "" {
			emails = append(emails, trimmed)
		}
	}
	if len(emails) > 0 {
		found := map[string]int{}
		for _, batch := range chunkStrings(emails, sqlInChunk) {
			var rows []model.ClientRecord
			if err := db.Select("id", "email").Where("email IN ?", batch).Find(&rows).Error; err != nil {
				return nil, err
			}
			for _, row := range rows {
				found[row.Email] = row.Id
			}
		}
		for _, email := range emails {
			id, ok := found[email]
			if !ok {
				return nil, common.NewError("client not found: " + email)
			}
			targets = append(targets, ExternalLinkTargetView{TargetType: model.ExternalLinkTargetClient, TargetId: id, Name: email})
		}
	}
	if group := strings.TrimSpace(req.Group); group != "" {
		var record model.ClientGroup
		err := db.Where("name = ?", group).First(&record).Error
		switch {
		case err == nil:
		case !createGroups:
			return nil, common.NewError("group not found: " + group)
		default:
			record = model.ClientGroup{Name: group}
			if err := db.Create(&record).Error; err != nil {
				return nil, err
			}
		}
		targets = append(targets, ExternalLinkTargetView{TargetType: model.ExternalLinkTargetGroup, TargetId: record.Id, Name: group})
	}
	if req.InboundId > 0 {
		var inbound model.Inbound
		if err := db.Select("id", "tag").Where("id = ?", req.InboundId).First(&inbound).Error; err != nil {
			return nil, common.NewError("inbound not found")
		}
		targets = append(targets, ExternalLinkTargetView{TargetType: model.ExternalLinkTargetInbound, TargetId: inbound.Id, Name: inbound.Tag})
	}
	if len(targets) == 0 {
		return nil, common.NewError("no target given: pass emails, group, inboundId, global or newClients")
	}
	return targets, nil
}
