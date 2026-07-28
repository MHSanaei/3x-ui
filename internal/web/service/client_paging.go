package service

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// ClientSlim is the row-shape used by the clients page. It drops fields the
// table never reads (UUID, password, auth, flow, security, reverse, tgId)
// so the list payload stays compact even when the panel manages thousands
// of clients. Modals that need the full record still call /get/:email.
type ClientSlim struct {
	Email      string              `json:"email"`
	SubID      string              `json:"subId"`
	Enable     bool                `json:"enable"`
	TotalGB    int64               `json:"totalGB"`
	ExpiryTime int64               `json:"expiryTime"`
	LimitIP    int                 `json:"limitIp"`
	Reset      int                 `json:"reset"`
	Group      string              `json:"group,omitempty"`
	Comment    string              `json:"comment,omitempty"`
	InboundIds []int               `json:"inboundIds"`
	Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
	CreatedAt  int64               `json:"createdAt"`
	UpdatedAt  int64               `json:"updatedAt"`
}

// ClientPageParams are the query params accepted by /panel/api/clients/list/paged.
// All fields are optional — the empty value means "no filter" / defaults.
//
// Filter / Protocol / Inbound accept either a single value or a comma-separated
// list; matching is OR within a field and AND across fields. The numeric range
// fields treat 0 as "unset" on the lower bound and 0 (or negative) as
// "unbounded" on the upper bound.
type ClientPageParams struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Search   string `form:"search"`
	Filter   string `form:"filter"`
	Protocol string `form:"protocol"`
	Inbound  string `form:"inbound"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`

	ExpiryFrom int64  `form:"expiryFrom"`
	ExpiryTo   int64  `form:"expiryTo"`
	UsageFrom  int64  `form:"usageFrom"`
	UsageTo    int64  `form:"usageTo"`
	AutoRenew  string `form:"autoRenew"`
	HasTgID    string `form:"hasTgId"`
	HasComment string `form:"hasComment"`
	Group      string `form:"group"`
}

// ClientPageResponse is the shape returned by ListPaged. `Total` is the
// row count in the DB; `Filtered` is the count after Search/Filter/Protocol
// were applied, before pagination. The page contains at most PageSize items.
// Summary is computed across the full DB row set so dashboard counters
// on the clients page stay stable as the user paginates/filters.
type ClientPageResponse struct {
	Items    []ClientSlim   `json:"items"`
	Total    int            `json:"total"`
	Filtered int            `json:"filtered"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Summary  ClientsSummary `json:"summary"`
	Groups   []string       `json:"groups"`
}

// ClientsSummary collects per-bucket counts plus the matching email lists so
// the clients page can render the dashboard stat cards and their hover
// popovers without shipping the full client array.
type ClientsSummary struct {
	Total    int      `json:"total"`
	Active   int      `json:"active"`
	Online   []string `json:"online"`
	Depleted []string `json:"depleted"`
	Expiring []string `json:"expiring"`
	Deactive []string `json:"deactive"`
}

const (
	clientPageDefaultSize = 25
	clientPageMaxSize     = 200
)

// ListPaged loads every client (with traffic + attachments) into memory,
// applies the requested filter / search / protocol predicates, sorts, and
// returns the requested page along with total and filtered counts. The DB
// query itself is unchanged from List(); the win is that the response
// only carries 25-ish slim rows over the wire instead of all 2000 full
// records, which on real panels was the dominant cost.
func (s *ClientService) ListPaged(inboundSvc *InboundService, settingSvc *SettingService, params ClientPageParams) (*ClientPageResponse, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	total := len(all)

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = clientPageDefaultSize
	}
	if pageSize > clientPageMaxSize {
		pageSize = clientPageMaxSize
	}
	page := params.Page
	if page <= 0 {
		page = 1
	}

	protocols := parseCSVStrings(params.Protocol)
	inboundIDs := parseCSVInts(params.Inbound)
	buckets := parseCSVStrings(params.Filter)

	var protocolByInbound map[int]string
	if len(protocols) > 0 {
		inbounds, err := inboundSvc.GetAllInbounds()
		if err == nil {
			protocolByInbound = make(map[int]string, len(inbounds))
			for _, ib := range inbounds {
				protocolByInbound[ib.Id] = string(ib.Protocol)
			}
		}
	}

	onlines := inboundSvc.GetOnlineClients()
	onlineSet := make(map[string]struct{}, len(onlines))
	for _, e := range onlines {
		onlineSet[e] = struct{}{}
	}

	var expireDiffMs, trafficDiffBytes int64
	if settingSvc != nil {
		if v, err := settingSvc.GetExpireDiff(); err == nil {
			expireDiffMs = int64(v) * 86400000
		}
		if v, err := settingSvc.GetTrafficDiff(); err == nil {
			trafficDiffBytes = int64(v) * 1073741824
		}
	}

	nowMs := time.Now().UnixMilli()
	summary := buildClientsSummary(all, onlineSet, nowMs, expireDiffMs, trafficDiffBytes)

	needle := strings.ToLower(strings.TrimSpace(params.Search))

	filtered := make([]ClientWithAttachments, 0, len(all))
	for _, c := range all {
		if needle != "" && !clientMatchesSearch(c, needle) {
			continue
		}
		if len(protocols) > 0 && !clientMatchesAnyProtocol(c, protocols, protocolByInbound) {
			continue
		}
		if len(inboundIDs) > 0 && !clientMatchesAnyInbound(c, inboundIDs) {
			continue
		}
		if len(buckets) > 0 && !clientMatchesAnyBucket(c, buckets, onlineSet, nowMs, expireDiffMs, trafficDiffBytes) {
			continue
		}
		if !clientMatchesExpiryRange(c, params.ExpiryFrom, params.ExpiryTo) {
			continue
		}
		if !clientMatchesUsageRange(c, params.UsageFrom, params.UsageTo) {
			continue
		}
		if !clientMatchesAutoRenew(c, params.AutoRenew) {
			continue
		}
		if !clientMatchesHasTgID(c, params.HasTgID) {
			continue
		}
		if !clientMatchesHasComment(c, params.HasComment) {
			continue
		}
		if !clientMatchesAnyGroup(c, params.Group) {
			continue
		}
		filtered = append(filtered, c)
	}

	sortClients(filtered, params.Sort, params.Order)

	filteredCount := len(filtered)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > filteredCount {
		start = filteredCount
	}
	if end > filteredCount {
		end = filteredCount
	}
	pageRows := filtered[start:end]

	items := make([]ClientSlim, 0, len(pageRows))
	for _, c := range pageRows {
		items = append(items, toClientSlim(c))
	}

	groupRows, gErr := s.ListGroups()
	if gErr != nil {
		return nil, gErr
	}
	groups := make([]string, 0, len(groupRows))
	for _, g := range groupRows {
		groups = append(groups, g.Name)
	}

	return &ClientPageResponse{
		Items:    items,
		Total:    total,
		Filtered: filteredCount,
		Page:     page,
		PageSize: pageSize,
		Summary:  summary,
		Groups:   groups,
	}, nil
}

func buildClientsSummary(all []ClientWithAttachments, onlineSet map[string]struct{}, nowMs, expireDiffMs, trafficDiffBytes int64) ClientsSummary {
	s := ClientsSummary{
		Total:    len(all),
		Online:   []string{},
		Depleted: []string{},
		Expiring: []string{},
		Deactive: []string{},
	}
	for _, c := range all {
		used := int64(0)
		if c.Traffic != nil {
			used = c.Traffic.Up + c.Traffic.Down
		}
		exhausted := c.TotalGB > 0 && used >= c.TotalGB
		expired := c.ExpiryTime > 0 && c.ExpiryTime <= nowMs
		if c.Enable {
			if _, ok := onlineSet[c.Email]; ok {
				s.Online = append(s.Online, c.Email)
			}
		}
		if exhausted || expired {
			s.Depleted = append(s.Depleted, c.Email)
			continue
		}
		if !c.Enable {
			s.Deactive = append(s.Deactive, c.Email)
			continue
		}
		nearExpiry := c.ExpiryTime > 0 && c.ExpiryTime-nowMs < expireDiffMs
		nearLimit := c.TotalGB > 0 && c.TotalGB-used < trafficDiffBytes
		if nearExpiry || nearLimit {
			s.Expiring = append(s.Expiring, c.Email)
		} else {
			s.Active++
		}
	}
	return s
}

func toClientSlim(c ClientWithAttachments) ClientSlim {
	return ClientSlim{
		Email:      c.Email,
		SubID:      c.SubID,
		Enable:     c.Enable,
		TotalGB:    c.TotalGB,
		ExpiryTime: c.ExpiryTime,
		LimitIP:    c.LimitIP,
		Reset:      c.Reset,
		Group:      c.Group,
		Comment:    c.Comment,
		InboundIds: c.InboundIds,
		Traffic:    c.Traffic,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

func clientMatchesSearch(c ClientWithAttachments, needle string) bool {
	if needle == "" {
		return true
	}
	candidates := [...]string{c.Email, c.SubID, c.Comment, c.UUID, c.Password, c.Auth}
	for _, v := range candidates {
		if v != "" && strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}
	if c.TgID != 0 && strings.Contains(strconv.FormatInt(c.TgID, 10), needle) {
		return true
	}
	return false
}

// parseCSVStrings splits a comma-separated list, trims/lower-cases each item,
// and drops blanks. Returns nil when the input has no usable entries — the
// caller can then skip the predicate entirely.
func parseCSVStrings(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.ToLower(strings.TrimSpace(p))
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseCSVInts is parseCSVStrings for positive integer IDs; non-numeric or
// non-positive entries are silently dropped.
func parseCSVInts(raw string) []int {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func clientMatchesAnyProtocol(c ClientWithAttachments, protocols []string, byInbound map[int]string) bool {
	for _, id := range c.InboundIds {
		p := byInbound[id]
		if p == "" {
			continue
		}
		if slices.Contains(protocols, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func clientMatchesAnyInbound(c ClientWithAttachments, inboundIds []int) bool {
	for _, id := range c.InboundIds {
		if slices.Contains(inboundIds, id) {
			return true
		}
	}
	return false
}

func clientMatchesAnyBucket(c ClientWithAttachments, buckets []string, onlineSet map[string]struct{}, nowMs, expireDiffMs, trafficDiffBytes int64) bool {
	for _, b := range buckets {
		if clientMatchesBucket(c, b, onlineSet, nowMs, expireDiffMs, trafficDiffBytes) {
			return true
		}
	}
	return false
}

func clientMatchesExpiryRange(c ClientWithAttachments, fromMs, toMs int64) bool {
	if fromMs <= 0 && toMs <= 0 {
		return true
	}
	// expiryTime of 0 means "never expires"; treat it as outside any bounded
	// range so users filtering by date see only clients with concrete expiries.
	if c.ExpiryTime == 0 {
		return false
	}
	// Negative expiry is the "delayed start" sentinel; same treatment as never.
	if c.ExpiryTime < 0 {
		return false
	}
	if fromMs > 0 && c.ExpiryTime < fromMs {
		return false
	}
	if toMs > 0 && c.ExpiryTime > toMs {
		return false
	}
	return true
}

func clientMatchesUsageRange(c ClientWithAttachments, fromBytes, toBytes int64) bool {
	if fromBytes <= 0 && toBytes <= 0 {
		return true
	}
	used := int64(0)
	if c.Traffic != nil {
		used = c.Traffic.Up + c.Traffic.Down
	}
	if fromBytes > 0 && used < fromBytes {
		return false
	}
	if toBytes > 0 && used > toBytes {
		return false
	}
	return true
}

func clientMatchesAutoRenew(c ClientWithAttachments, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on":
		return c.Reset > 0
	case "off":
		return c.Reset <= 0
	}
	return true
}

func clientMatchesHasTgID(c ClientWithAttachments, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "yes":
		return c.TgID != 0
	case "no":
		return c.TgID == 0
	}
	return true
}

func clientMatchesHasComment(c ClientWithAttachments, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "yes":
		return strings.TrimSpace(c.Comment) != ""
	case "no":
		return strings.TrimSpace(c.Comment) == ""
	}
	return true
}

func clientMatchesAnyGroup(c ClientWithAttachments, csv string) bool {
	groups := parseCSVStrings(csv)
	if len(groups) == 0 {
		return true
	}
	current := strings.TrimSpace(c.Group)
	for _, g := range groups {
		if g == "" {
			if current == "" {
				return true
			}
			continue
		}
		if strings.EqualFold(g, current) {
			return true
		}
	}
	return false
}

func clientMatchesBucket(c ClientWithAttachments, bucket string, onlineSet map[string]struct{}, nowMs, expireDiffMs, trafficDiffBytes int64) bool {
	if bucket == "" {
		return true
	}
	used := int64(0)
	if c.Traffic != nil {
		used = c.Traffic.Up + c.Traffic.Down
	}
	exhausted := c.TotalGB > 0 && used >= c.TotalGB
	expired := c.ExpiryTime > 0 && c.ExpiryTime <= nowMs
	switch bucket {
	case "online":
		if onlineSet == nil {
			return false
		}
		_, ok := onlineSet[c.Email]
		return ok && c.Enable
	case "depleted":
		return exhausted || expired
	case "deactive":
		return !c.Enable
	case "active":
		return c.Enable && !exhausted && !expired
	case "expiring":
		if !c.Enable || exhausted || expired {
			return false
		}
		nearExpiry := c.ExpiryTime > 0 && c.ExpiryTime-nowMs < expireDiffMs
		nearLimit := c.TotalGB > 0 && c.TotalGB-used < trafficDiffBytes
		return nearExpiry || nearLimit
	}
	return true
}

func sortClients(rows []ClientWithAttachments, sortKey, order string) {
	if sortKey == "" {
		return
	}
	desc := order == "descend"
	less := func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch sortKey {
		case "enable":
			if a.Enable == b.Enable {
				return false
			}
			return !a.Enable && b.Enable
		case "email":
			return strings.ToLower(a.Email) < strings.ToLower(b.Email)
		case "inboundIds":
			return len(a.InboundIds) < len(b.InboundIds)
		case "traffic":
			ua := int64(0)
			if a.Traffic != nil {
				ua = a.Traffic.Up + a.Traffic.Down
			}
			ub := int64(0)
			if b.Traffic != nil {
				ub = b.Traffic.Up + b.Traffic.Down
			}
			return ua < ub
		case "remaining":
			ra := int64(1<<62 - 1)
			if a.TotalGB > 0 {
				used := int64(0)
				if a.Traffic != nil {
					used = a.Traffic.Up + a.Traffic.Down
				}
				ra = a.TotalGB - used
			}
			rb := int64(1<<62 - 1)
			if b.TotalGB > 0 {
				used := int64(0)
				if b.Traffic != nil {
					used = b.Traffic.Up + b.Traffic.Down
				}
				rb = b.TotalGB - used
			}
			return ra < rb
		case "expiryTime":
			ea := int64(1<<62 - 1)
			if a.ExpiryTime > 0 {
				ea = a.ExpiryTime
			}
			eb := int64(1<<62 - 1)
			if b.ExpiryTime > 0 {
				eb = b.ExpiryTime
			}
			return ea < eb
		case "createdAt":
			if a.CreatedAt == b.CreatedAt {
				return a.Id < b.Id
			}
			return a.CreatedAt < b.CreatedAt
		case "updatedAt":
			if a.UpdatedAt == b.UpdatedAt {
				return a.Id < b.Id
			}
			return a.UpdatedAt < b.UpdatedAt
		case "lastOnline":
			la := int64(0)
			if a.Traffic != nil {
				la = a.Traffic.LastOnline
			}
			lb := int64(0)
			if b.Traffic != nil {
				lb = b.Traffic.LastOnline
			}
			if la == lb {
				return a.Id < b.Id
			}
			return la < lb
		}
		return false
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if desc {
			return less(j, i)
		}
		return less(i, j)
	})
}
