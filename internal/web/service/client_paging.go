package service

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
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
	LimitHwid  int                 `json:"limitHwid"`
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
// popovers without shipping the full client array. The counters are exact;
// the lists stop at clientSummaryEmailCap entries and only back the popovers.
type ClientsSummary struct {
	Total         int      `json:"total"`
	Active        int      `json:"active"`
	OnlineCount   int      `json:"onlineCount"`
	DepletedCount int      `json:"depletedCount"`
	ExpiringCount int      `json:"expiringCount"`
	DeactiveCount int      `json:"deactiveCount"`
	Online        []string `json:"online"`
	Depleted      []string `json:"depleted"`
	Expiring      []string `json:"expiring"`
	Deactive      []string `json:"deactive"`
}

const (
	clientPageDefaultSize = 25
	clientPageMaxSize     = 200
	// clientSummaryEmailCap bounds each bucket's email list. Shipping every
	// matching email made the response — and the Zod validation the page runs
	// over it — grow with the client count on a request that repeats every 5s,
	// and left the hover popover rendering thousands of rows.
	clientSummaryEmailCap = 200
	// sqlNeverSentinel sorts "never expires" / "unlimited quota" clients last,
	// matching the sentinel the in-memory comparator used.
	sqlNeverSentinel = "4611686018427387903"
	// sqlClientEnabled tolerates a NULL enable column, which GORM scans as
	// false: without the COALESCE such a row would match neither the enabled
	// nor the disabled branch of any predicate.
	sqlClientEnabled = "COALESCE(c.enable, FALSE)"
)

const clientSearchCond = `(LOWER(c.email) LIKE ? ESCAPE '\'
	OR LOWER(COALESCE(c.sub_id, '')) LIKE ? ESCAPE '\'
	OR LOWER(COALESCE(c.comment, '')) LIKE ? ESCAPE '\'
	OR LOWER(COALESCE(c.uuid, '')) LIKE ? ESCAPE '\'
	OR LOWER(COALESCE(c.password, '')) LIKE ? ESCAPE '\'
	OR LOWER(COALESCE(c.auth, '')) LIKE ? ESCAPE '\'
	OR (COALESCE(c.tg_id, 0) <> 0 AND CAST(c.tg_id AS TEXT) LIKE ? ESCAPE '\'))`

// clientQuery builds the statements behind the clients page: a clients row
// joined to its traffic counters, plus the expressions every bucket predicate
// shares. Filtering, sorting, paging and the summary all run in the database.
// Loading every client (with attachments and traffic) into Go and doing it in
// memory cost ~200ms per request at 20k clients on a page that polls every
// 5 seconds, which is what made the table feel stuck on large panels.
type clientQuery struct {
	db               *gorm.DB
	joins            []clientQueryJoin
	usedExpr         string
	nowMs            int64
	expireDiffMs     int64
	trafficDiffBytes int64
}

type clientQueryJoin struct {
	sql  string
	args []any
}

func newClientQuery(db *gorm.DB, nowMs, expireDiffMs, trafficDiffBytes int64) clientQuery {
	q := clientQuery{
		db:               db,
		nowMs:            nowMs,
		expireDiffMs:     expireDiffMs,
		trafficDiffBytes: trafficDiffBytes,
		joins:            []clientQueryJoin{{sql: "LEFT JOIN client_traffics ct ON ct.email = c.email"}},
		usedExpr:         "(COALESCE(ct.up, 0) + COALESCE(ct.down, 0))",
	}
	freshSince := globalTrafficFreshSince()
	var probe int64
	err := db.Model(&model.ClientGlobalTraffic{}).
		Where("updated_at >= ?", freshSince).
		Limit(1).Count(&probe).Error
	if err != nil || probe == 0 {
		return q
	}
	// A master still pushes cross-panel usage here, so the predicates have to
	// see the same raised counters overlayGlobalTraffic applies on read.
	q.joins = append(q.joins, clientQueryJoin{
		sql: "LEFT JOIN (SELECT email, MAX(up) AS up, MAX(down) AS down FROM client_global_traffics" +
			" WHERE updated_at >= ? GROUP BY email) g ON g.email = c.email",
		args: []any{freshSince},
	})
	q.usedExpr = "(CASE WHEN COALESCE(g.up, 0) > COALESCE(ct.up, 0) THEN COALESCE(g.up, 0) ELSE COALESCE(ct.up, 0) END" +
		" + CASE WHEN COALESCE(g.down, 0) > COALESCE(ct.down, 0) THEN COALESCE(g.down, 0) ELSE COALESCE(ct.down, 0) END)"
	return q
}

func (q clientQuery) from() *gorm.DB {
	tx := q.db.Table("clients AS c")
	for _, j := range q.joins {
		tx = tx.Joins(j.sql, j.args...)
	}
	return tx
}

func (q clientQuery) depletedExpr() string {
	return "((c.total_gb > 0 AND " + q.usedExpr + " >= c.total_gb)" +
		" OR (c.expiry_time > 0 AND c.expiry_time <= " + sqlInt(q.nowMs) + "))"
}

func (q clientQuery) nearDepletionExpr() string {
	return "((c.expiry_time > 0 AND c.expiry_time - " + sqlInt(q.nowMs) + " < " + sqlInt(q.expireDiffMs) + ")" +
		" OR (c.total_gb > 0 AND c.total_gb - " + q.usedExpr + " < " + sqlInt(q.trafficDiffBytes) + "))"
}

func (q clientQuery) expiringExpr() string {
	return "(" + sqlClientEnabled + " AND NOT " + q.depletedExpr() + " AND " + q.nearDepletionExpr() + ")"
}

func (q clientQuery) activeExpr() string {
	return "(" + sqlClientEnabled + " AND NOT " + q.depletedExpr() + " AND NOT " + q.nearDepletionExpr() + ")"
}

// summaryDeactiveExpr is narrower than the "deactive" bucket filter: a disabled
// client that also ran out counts once, under depleted, so the stat cards add
// up to the client total.
func (q clientQuery) summaryDeactiveExpr() string {
	return "(NOT " + sqlClientEnabled + " AND NOT " + q.depletedExpr() + ")"
}

// applyParams narrows tx by every predicate the clients page sends. Matching is
// OR within a field and AND across fields, mirroring the query-param contract.
// The second return says whether anything narrowed the set, so an unfiltered
// request can reuse the total count instead of scanning for it again.
func (q clientQuery) applyParams(tx *gorm.DB, params ClientPageParams, onlines []string) (*gorm.DB, bool) {
	narrowed := false
	where := func(cond string, args ...any) {
		narrowed = true
		tx = tx.Where(cond, args...)
	}

	if needle := strings.ToLower(strings.TrimSpace(params.Search)); needle != "" {
		pattern := "%" + escapeLikeLiteral(needle) + "%"
		where(clientSearchCond, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if protocols := parseCSVStrings(params.Protocol); len(protocols) > 0 {
		where("EXISTS (SELECT 1 FROM client_inbounds ci JOIN inbounds ib ON ib.id = ci.inbound_id"+
			" WHERE ci.client_id = c.id AND LOWER(ib.protocol) IN ?)", protocols)
	}
	if inboundIds := parseCSVInts(params.Inbound); len(inboundIds) > 0 {
		where("EXISTS (SELECT 1 FROM client_inbounds ci WHERE ci.client_id = c.id AND ci.inbound_id IN ?)", inboundIds)
	}
	if buckets := parseCSVStrings(params.Filter); len(buckets) > 0 {
		cond, args := q.bucketCond(buckets, onlines)
		where(cond, args...)
	}
	if params.ExpiryFrom > 0 || params.ExpiryTo > 0 {
		// 0 means "never expires" and a negative value is the delayed-start
		// sentinel; both sit outside any bounded range.
		where("c.expiry_time > 0")
		if params.ExpiryFrom > 0 {
			where("c.expiry_time >= ?", params.ExpiryFrom)
		}
		if params.ExpiryTo > 0 {
			where("c.expiry_time <= ?", params.ExpiryTo)
		}
	}
	if params.UsageFrom > 0 {
		where(q.usedExpr+" >= ?", params.UsageFrom)
	}
	if params.UsageTo > 0 {
		where(q.usedExpr+" <= ?", params.UsageTo)
	}
	switch strings.ToLower(strings.TrimSpace(params.AutoRenew)) {
	case "on":
		where("COALESCE(c.reset, 0) > 0")
	case "off":
		where("COALESCE(c.reset, 0) <= 0")
	}
	switch strings.ToLower(strings.TrimSpace(params.HasTgID)) {
	case "yes":
		where("COALESCE(c.tg_id, 0) <> 0")
	case "no":
		where("COALESCE(c.tg_id, 0) = 0")
	}
	switch strings.ToLower(strings.TrimSpace(params.HasComment)) {
	case "yes":
		where("TRIM(COALESCE(c.comment, '')) <> ''")
	case "no":
		where("TRIM(COALESCE(c.comment, '')) = ''")
	}
	if groups := parseCSVStrings(params.Group); len(groups) > 0 {
		where("LOWER(TRIM(COALESCE(c.group_name, ''))) IN ?", groups)
	}
	return tx, narrowed
}

func (q clientQuery) bucketCond(buckets, onlines []string) (string, []any) {
	conds := make([]string, 0, len(buckets))
	args := make([]any, 0, len(buckets))
	for _, b := range buckets {
		switch b {
		case "active":
			conds = append(conds, "("+sqlClientEnabled+" AND NOT "+q.depletedExpr()+")")
		case "deactive":
			conds = append(conds, "(NOT "+sqlClientEnabled+")")
		case "depleted":
			conds = append(conds, q.depletedExpr())
		case "expiring":
			conds = append(conds, q.expiringExpr())
		case "online":
			cond, inArgs := emailInCond("c.email", onlines)
			conds = append(conds, "("+sqlClientEnabled+" AND "+cond+")")
			args = append(args, inArgs...)
		default:
			// An unrecognised bucket name matched every client before the
			// predicates moved into SQL; keep that so a stale saved filter
			// cannot silently empty the table.
			conds = append(conds, "(1 = 1)")
		}
	}
	return "(" + strings.Join(conds, " OR ") + ")", args
}

func (q clientQuery) applyOrder(tx *gorm.DB, sortKey, order string) *gorm.DB {
	dir := " ASC"
	if order == "descend" {
		dir = " DESC"
	}
	// createdAt / updatedAt / lastOnline broke ties on the client id inside the
	// comparator, so reversing the sort reversed the tiebreak with it. The
	// other keys leaned on a stable sort over an id-ordered slice instead.
	tieDir := " ASC"
	var expr string
	switch sortKey {
	case "enable":
		expr = sqlClientEnabled
	case "email":
		expr = "LOWER(c.email)"
	case "inboundIds":
		expr = "(SELECT COUNT(*) FROM client_inbounds ci WHERE ci.client_id = c.id)"
	case "traffic":
		expr = q.usedExpr
	case "remaining":
		expr = "CASE WHEN c.total_gb > 0 THEN c.total_gb - " + q.usedExpr + " ELSE " + sqlNeverSentinel + " END"
	case "expiryTime":
		expr = "CASE WHEN c.expiry_time > 0 THEN c.expiry_time ELSE " + sqlNeverSentinel + " END"
	case "createdAt":
		expr, tieDir = "c.created_at", dir
	case "updatedAt":
		expr, tieDir = "c.updated_at", dir
	case "lastOnline":
		expr, tieDir = "COALESCE(ct.last_online, 0)", dir
	default:
		return tx.Order("c.id ASC")
	}
	return tx.Order(expr + dir + ", c.id" + tieDir)
}

// ListPaged returns one page of clients together with the counts the clients
// page header needs. Every predicate runs in SQL, so the cost tracks the page
// size rather than the number of clients on the panel.
func (s *ClientService) ListPaged(inboundSvc *InboundService, settingSvc *SettingService, params ClientPageParams) (*ClientPageResponse, error) {
	db := database.GetDB()

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

	var expireDiffMs, trafficDiffBytes int64
	if settingSvc != nil {
		if v, err := settingSvc.GetExpireDiff(); err == nil {
			expireDiffMs = int64(v) * 86400000
		}
		if v, err := settingSvc.GetTrafficDiff(); err == nil {
			trafficDiffBytes = int64(v) * 1073741824
		}
	}

	onlines := inboundSvc.GetOnlineClients()
	q := newClientQuery(db, time.Now().UnixMilli(), expireDiffMs, trafficDiffBytes)

	var total int64
	if err := db.Model(&model.ClientRecord{}).Count(&total).Error; err != nil {
		return nil, err
	}

	summary, err := q.summary(onlines, int(total))
	if err != nil {
		return nil, err
	}

	filtered := total
	if scoped, narrowed := q.applyParams(q.from(), params, onlines); narrowed {
		if err := scoped.Count(&filtered).Error; err != nil {
			return nil, err
		}
	}

	items := []ClientSlim{}
	offset := (page - 1) * pageSize
	if int64(offset) < filtered {
		items, err = q.pageRows(params, onlines, offset, pageSize)
		if err != nil {
			return nil, err
		}
	}

	groups, err := s.listGroupNames()
	if err != nil {
		return nil, err
	}

	return &ClientPageResponse{
		Items:    items,
		Total:    int(total),
		Filtered: int(filtered),
		Page:     page,
		PageSize: pageSize,
		Summary:  summary,
		Groups:   groups,
	}, nil
}

// pageRows resolves the requested page to client ids, then loads the records,
// attachments and traffic for those ids only. A page never exceeds
// clientPageMaxSize rows, which stays under sqlInChunk, so the follow-up IN
// lists need no chunking.
func (q clientQuery) pageRows(params ClientPageParams, onlines []string, offset, limit int) ([]ClientSlim, error) {
	tx, _ := q.applyParams(q.from(), params, onlines)
	var ids []int
	if err := q.applyOrder(tx, params.Sort, params.Order).
		Offset(offset).Limit(limit).
		Pluck("c.id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []ClientSlim{}, nil
	}

	var records []model.ClientRecord
	if err := q.db.Where("id IN ?", ids).Find(&records).Error; err != nil {
		return nil, err
	}
	byId := make(map[int]*model.ClientRecord, len(records))
	emails := make([]string, 0, len(records))
	for i := range records {
		byId[records[i].Id] = &records[i]
		if records[i].Email != "" {
			emails = append(emails, records[i].Email)
		}
	}

	var links []model.ClientInbound
	if err := q.db.Where("client_id IN ?", ids).Order("inbound_id ASC").Find(&links).Error; err != nil {
		return nil, err
	}
	attachments := make(map[int][]int, len(ids))
	for _, l := range links {
		attachments[l.ClientId] = append(attachments[l.ClientId], l.InboundId)
	}

	trafficByEmail := make(map[string]*xray.ClientTraffic, len(emails))
	if len(emails) > 0 {
		var stats []xray.ClientTraffic
		if err := q.db.Where("email IN ?", emails).Find(&stats).Error; err != nil {
			return nil, err
		}
		overlayGlobalTrafficValues(q.db, stats)
		for i := range stats {
			trafficByEmail[stats[i].Email] = &stats[i]
		}
	}

	items := make([]ClientSlim, 0, len(ids))
	for _, id := range ids {
		rec := byId[id]
		if rec == nil {
			continue
		}
		items = append(items, toClientSlim(ClientWithAttachments{
			ClientRecord: *rec,
			InboundIds:   attachments[rec.Id],
			Traffic:      trafficByEmail[rec.Email],
		}))
	}
	return items, nil
}

func (q clientQuery) summary(onlines []string, total int) (ClientsSummary, error) {
	s := ClientsSummary{
		Total:    total,
		Online:   []string{},
		Depleted: []string{},
		Expiring: []string{},
		Deactive: []string{},
	}

	var counts struct {
		Active   int64
		Depleted int64
		Expiring int64
		Deactive int64
	}
	// SUM over an empty table yields NULL, which not every driver scans into an
	// int; COALESCE keeps a panel with no clients from erroring out.
	if err := q.from().Select(
		"COALESCE(SUM(CASE WHEN " + q.activeExpr() + " THEN 1 ELSE 0 END), 0) AS active," +
			" COALESCE(SUM(CASE WHEN " + q.depletedExpr() + " THEN 1 ELSE 0 END), 0) AS depleted," +
			" COALESCE(SUM(CASE WHEN " + q.expiringExpr() + " THEN 1 ELSE 0 END), 0) AS expiring," +
			" COALESCE(SUM(CASE WHEN " + q.summaryDeactiveExpr() + " THEN 1 ELSE 0 END), 0) AS deactive",
	).Scan(&counts).Error; err != nil {
		return s, err
	}
	s.Active = int(counts.Active)
	s.DepletedCount = int(counts.Depleted)
	s.ExpiringCount = int(counts.Expiring)
	s.DeactiveCount = int(counts.Deactive)

	buckets := []struct {
		cond  string
		count int
		out   *[]string
	}{
		{q.depletedExpr(), s.DepletedCount, &s.Depleted},
		{q.expiringExpr(), s.ExpiringCount, &s.Expiring},
		{q.summaryDeactiveExpr(), s.DeactiveCount, &s.Deactive},
	}
	for _, b := range buckets {
		// The counter already says the bucket is empty, so skip the scan that
		// would look for emails it cannot find.
		if b.count == 0 {
			continue
		}
		var emails []string
		if err := q.from().Where(b.cond).
			Order("c.id ASC").Limit(clientSummaryEmailCap).
			Pluck("c.email", &emails).Error; err != nil {
			return s, err
		}
		if len(emails) > 0 {
			*b.out = emails
		}
	}

	online, onlineCount, err := q.onlineEmails(onlines)
	if err != nil {
		return s, err
	}
	s.Online = online
	s.OnlineCount = onlineCount
	return s, nil
}

// onlineEmails intersects the emails xray reports as connected with the enabled
// clients this panel stores. The online set lives in memory and is bounded by
// live connections, so it drives the query rather than a scan of every client.
func (q clientQuery) onlineEmails(onlines []string) ([]string, int, error) {
	matched := []string{}
	count := 0
	for _, batch := range chunkStrings(onlines, sqlInChunk) {
		var page []string
		if err := q.db.Model(&model.ClientRecord{}).
			Where("COALESCE(enable, FALSE) = TRUE AND email IN ?", batch).
			Order("id ASC").
			Pluck("email", &page).Error; err != nil {
			return nil, 0, err
		}
		count += len(page)
		if room := clientSummaryEmailCap - len(matched); room > 0 {
			matched = append(matched, page[:min(room, len(page))]...)
		}
	}
	return matched, count, nil
}

// listGroupNames returns the group names the clients page offers as filters:
// the stored groups plus any name a client still carries. ListGroups also sums
// per-client traffic per group, which this page never reads and which costs a
// full join over client_traffics on every poll.
func (s *ClientService) listGroupNames() ([]string, error) {
	db := database.GetDB()
	var stored []string
	if err := db.Model(&model.ClientGroup{}).Pluck("name", &stored).Error; err != nil {
		return nil, err
	}
	var used []string
	if err := db.Model(&model.ClientRecord{}).
		Where("group_name <> ''").
		Distinct().
		Pluck("group_name", &used).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(stored)+len(used))
	out := make([]string, 0, len(stored)+len(used))
	for _, list := range [][]string{stored, used} {
		for _, name := range list {
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out, nil
}

func sqlInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func toClientSlim(c ClientWithAttachments) ClientSlim {
	return ClientSlim{
		Email:      c.Email,
		SubID:      c.SubID,
		Enable:     c.Enable,
		TotalGB:    c.TotalGB,
		ExpiryTime: c.ExpiryTime,
		LimitIP:    c.LimitIP,
		LimitHwid:  c.LimitHwid,
		Reset:      c.Reset,
		Group:      c.Group,
		Comment:    c.Comment,
		InboundIds: c.InboundIds,
		Traffic:    c.Traffic,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

// escapeLikeLiteral neutralises LIKE wildcards so searching for "a_b" keeps
// matching literally, the way strings.Contains did.
func escapeLikeLiteral(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// emailInCond renders an IN over a possibly large email set, split so no single
// IN list outgrows the drivers' bind-parameter ceiling.
func emailInCond(column string, emails []string) (string, []any) {
	if len(emails) == 0 {
		return "1 = 0", nil
	}
	chunks := chunkStrings(emails, sqlInChunk)
	parts := make([]string, 0, len(chunks))
	args := make([]any, 0, len(chunks))
	for _, chunk := range chunks {
		parts = append(parts, column+" IN ?")
		args = append(args, chunk)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
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
