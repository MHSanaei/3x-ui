package service

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	pagingDay = int64(86400000)
	pagingGB  = int64(1) << 30
)

type pagingSeed struct {
	email      string
	enable     bool
	totalGB    int64
	expiryTime int64
	used       int64
	subID      string
	uuid       string
	password   string
	auth       string
	comment    string
	group      string
	tgID       int64
	reset      int
	lastOnline int64
	inbounds   []int
}

// seedPagingClients writes one vless and one trojan inbound plus a fixed client
// set covering every bucket, sort key and search field ListPaged supports.
// Returns "now" so the expectations can be phrased relative to it.
func seedPagingClients(t *testing.T) (int64, []pagingSeed) {
	t.Helper()
	db := database.GetDB()
	now := time.Now().UnixMilli()

	vless := &model.Inbound{UserId: 1, Tag: "in-vless", Enable: true, Port: 40001, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	trojan := &model.Inbound{UserId: 1, Tag: "in-trojan", Enable: true, Port: 40002, Protocol: model.Trojan, Settings: `{"clients":[]}`}
	for _, ib := range []*model.Inbound{vless, trojan} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	seeds := []pagingSeed{
		{email: "alpha@x", enable: true, totalGB: 0, expiryTime: 0, used: 5 * pagingGB, subID: "sub-alpha", uuid: "uuid-alpha", inbounds: []int{vless.Id}, lastOnline: now - 10*pagingDay},
		{email: "bravo@x", enable: true, totalGB: 10 * pagingGB, expiryTime: now + 30*pagingDay, used: pagingGB, password: "pw-bravo", inbounds: []int{vless.Id, trojan.Id}, lastOnline: now - pagingDay},
		{email: "charlie@x", enable: true, totalGB: 10 * pagingGB, expiryTime: now + 30*pagingDay, used: 10 * pagingGB, auth: "auth-charlie", inbounds: []int{trojan.Id}},
		{email: "delta@x", enable: true, totalGB: 10 * pagingGB, expiryTime: now - pagingDay, used: pagingGB, inbounds: []int{vless.Id}},
		{email: "echo@x", enable: false, totalGB: 10 * pagingGB, expiryTime: now + 30*pagingDay, inbounds: []int{vless.Id}},
		{email: "foxtrot@x", enable: false, totalGB: 10 * pagingGB, expiryTime: now - pagingDay, inbounds: []int{trojan.Id}},
		{email: "golf@x", enable: true, totalGB: 10 * pagingGB, expiryTime: now + 2*pagingDay, inbounds: []int{vless.Id}},
		{email: "hotel@x", enable: true, totalGB: 10 * pagingGB, expiryTime: now + 30*pagingDay, used: 10*pagingGB - pagingGB/2, inbounds: []int{vless.Id}},
		{email: "india@x", enable: true, totalGB: 0, expiryTime: -5 * pagingDay, inbounds: []int{vless.Id}},
		{email: "juliet@x", enable: true, comment: " vip customer ", group: "VIP", tgID: 555, reset: 7, inbounds: []int{vless.Id}},
		{email: "kilo_1@x", enable: true, group: "vip", inbounds: nil},
		{email: "kilo1@x", enable: true, inbounds: []int{trojan.Id}},
	}

	for i, s := range seeds {
		rec := model.ClientRecord{
			Email:      s.email,
			SubID:      s.subID,
			UUID:       s.uuid,
			Password:   s.password,
			Auth:       s.auth,
			Comment:    s.comment,
			Group:      s.group,
			TgID:       s.tgID,
			Reset:      s.reset,
			Enable:     s.enable,
			TotalGB:    s.totalGB,
			ExpiryTime: s.expiryTime,
			CreatedAt:  now - int64(len(seeds)-i)*pagingDay,
			UpdatedAt:  now - int64(i)*pagingDay,
		}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("create client %s: %v", s.email, err)
		}
		if !s.enable {
			// clients.enable carries a `default:true` tag, so GORM leaves the
			// zero value out of the INSERT and the column comes back true.
			// Restate updated_at so the autoUpdateTime hook cannot reshuffle
			// the sort fixtures.
			if err := db.Model(&model.ClientRecord{}).Where("id = ?", rec.Id).
				Updates(map[string]any{"enable": false, "updated_at": rec.UpdatedAt}).Error; err != nil {
				t.Fatalf("disable %s: %v", s.email, err)
			}
		}
		traffic := xray.ClientTraffic{
			Email:      s.email,
			Enable:     s.enable,
			Up:         s.used / 2,
			Down:       s.used - s.used/2,
			Total:      s.totalGB,
			ExpiryTime: s.expiryTime,
			LastOnline: s.lastOnline,
		}
		if err := db.Create(&traffic).Error; err != nil {
			t.Fatalf("create traffic %s: %v", s.email, err)
		}
		for _, id := range s.inbounds {
			if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: id}).Error; err != nil {
				t.Fatalf("attach %s to %d: %v", s.email, id, err)
			}
		}
	}
	return now, seeds
}

func pagedEmails(items []ClientSlim) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Email)
	}
	return out
}

func setupPagingServices(t *testing.T) (*ClientService, *InboundService, *SettingService) {
	t.Helper()
	setupBulkDB(t)
	settingSvc := &SettingService{}
	if err := settingSvc.setInt("expireDiff", 3); err != nil {
		t.Fatalf("set expireDiff: %v", err)
	}
	if err := settingSvc.setInt("trafficDiff", 1); err != nil {
		t.Fatalf("set trafficDiff: %v", err)
	}
	return &ClientService{}, &InboundService{}, settingSvc
}

func TestListPagedFilters(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	now, _ := seedPagingClients(t)

	tests := []struct {
		name   string
		params ClientPageParams
		want   []string
	}{
		{
			name:   "no filter returns every client in id order",
			params: ClientPageParams{PageSize: 50},
			want:   []string{"alpha@x", "bravo@x", "charlie@x", "delta@x", "echo@x", "foxtrot@x", "golf@x", "hotel@x", "india@x", "juliet@x", "kilo_1@x", "kilo1@x"},
		},
		{
			name:   "depleted bucket covers quota and expiry",
			params: ClientPageParams{PageSize: 50, Filter: "depleted"},
			want:   []string{"charlie@x", "delta@x", "foxtrot@x"},
		},
		{
			name:   "deactive bucket is every disabled client",
			params: ClientPageParams{PageSize: 50, Filter: "deactive"},
			want:   []string{"echo@x", "foxtrot@x"},
		},
		{
			name:   "expiring bucket covers near expiry and near quota",
			params: ClientPageParams{PageSize: 50, Filter: "expiring"},
			want:   []string{"golf@x", "hotel@x"},
		},
		{
			name:   "active bucket keeps enabled clients that still have room",
			params: ClientPageParams{PageSize: 50, Filter: "active"},
			want:   []string{"alpha@x", "bravo@x", "golf@x", "hotel@x", "india@x", "juliet@x", "kilo_1@x", "kilo1@x"},
		},
		{
			name:   "buckets are ORed",
			params: ClientPageParams{PageSize: 50, Filter: "depleted,expiring"},
			want:   []string{"charlie@x", "delta@x", "foxtrot@x", "golf@x", "hotel@x"},
		},
		{
			name:   "unknown bucket keeps matching everything",
			params: ClientPageParams{PageSize: 50, Filter: "nonsense"},
			want:   []string{"alpha@x", "bravo@x", "charlie@x", "delta@x", "echo@x", "foxtrot@x", "golf@x", "hotel@x", "india@x", "juliet@x", "kilo_1@x", "kilo1@x"},
		},
		{
			name:   "protocol filter follows the attachments",
			params: ClientPageParams{PageSize: 50, Protocol: "trojan"},
			want:   []string{"bravo@x", "charlie@x", "foxtrot@x", "kilo1@x"},
		},
		{
			name:   "inbound filter follows the attachments",
			params: ClientPageParams{PageSize: 50, Inbound: "2"},
			want:   []string{"bravo@x", "charlie@x", "foxtrot@x", "kilo1@x"},
		},
		{
			name:   "search matches the email",
			params: ClientPageParams{PageSize: 50, Search: "KILO"},
			want:   []string{"kilo_1@x", "kilo1@x"},
		},
		{
			name:   "search treats LIKE wildcards literally",
			params: ClientPageParams{PageSize: 50, Search: "kilo_1"},
			want:   []string{"kilo_1@x"},
		},
		{
			name:   "search matches the subId",
			params: ClientPageParams{PageSize: 50, Search: "sub-alpha"},
			want:   []string{"alpha@x"},
		},
		{
			name:   "search matches the uuid",
			params: ClientPageParams{PageSize: 50, Search: "uuid-alpha"},
			want:   []string{"alpha@x"},
		},
		{
			name:   "search matches the password",
			params: ClientPageParams{PageSize: 50, Search: "pw-bravo"},
			want:   []string{"bravo@x"},
		},
		{
			name:   "search matches the auth",
			params: ClientPageParams{PageSize: 50, Search: "auth-charlie"},
			want:   []string{"charlie@x"},
		},
		{
			name:   "search matches the comment",
			params: ClientPageParams{PageSize: 50, Search: "vip customer"},
			want:   []string{"juliet@x"},
		},
		{
			name:   "search matches the telegram id",
			params: ClientPageParams{PageSize: 50, Search: "555"},
			want:   []string{"juliet@x"},
		},
		{
			name:   "group filter is case insensitive",
			params: ClientPageParams{PageSize: 50, Group: "vip"},
			want:   []string{"juliet@x", "kilo_1@x"},
		},
		{
			name:   "hasComment yes",
			params: ClientPageParams{PageSize: 50, HasComment: "yes"},
			want:   []string{"juliet@x"},
		},
		{
			name:   "hasTgId yes",
			params: ClientPageParams{PageSize: 50, HasTgID: "yes"},
			want:   []string{"juliet@x"},
		},
		{
			name:   "autoRenew on",
			params: ClientPageParams{PageSize: 50, AutoRenew: "on"},
			want:   []string{"juliet@x"},
		},
		{
			name:   "usage range is inclusive on both bounds",
			params: ClientPageParams{PageSize: 50, UsageFrom: pagingGB, UsageTo: 5 * pagingGB},
			want:   []string{"alpha@x", "bravo@x", "delta@x"},
		},
		{
			name:   "expiry range excludes never and delayed start",
			params: ClientPageParams{PageSize: 50, ExpiryFrom: now, ExpiryTo: now + 10*pagingDay},
			want:   []string{"golf@x"},
		},
		{
			name:   "filters combine with AND",
			params: ClientPageParams{PageSize: 50, Filter: "depleted", Protocol: "trojan"},
			want:   []string{"charlie@x", "foxtrot@x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := svc.ListPaged(inboundSvc, settingSvc, tc.params)
			if err != nil {
				t.Fatalf("ListPaged: %v", err)
			}
			got := pagedEmails(resp.Items)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("emails = %v, want %v", got, tc.want)
			}
			if resp.Filtered != len(tc.want) {
				t.Fatalf("filtered = %d, want %d", resp.Filtered, len(tc.want))
			}
			if resp.Total != 12 {
				t.Fatalf("total = %d, want 12", resp.Total)
			}
		})
	}
}

func TestListPagedSourceFilter(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	seedPagingClients(t)
	db := database.GetDB()

	relay := &model.Inbound{
		UserId:   1,
		Tag:      "relay-in-40100",
		Enable:   true,
		Port:     40100,
		Protocol: model.VLESS,
		Settings: "{\"clients\":[]}",
	}
	if err := db.Create(relay).Error; err != nil {
		t.Fatalf("create relay inbound: %v", err)
	}
	var standalone model.ClientRecord
	if err := db.Where("email = ?", "kilo_1@x").First(&standalone).Error; err != nil {
		t.Fatalf("find standalone client: %v", err)
	}
	var relayClient model.ClientRecord
	if err := db.Where("email = ?", "juliet@x").First(&relayClient).Error; err != nil {
		t.Fatalf("find relay client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: relayClient.Id, InboundId: relay.Id}).Error; err != nil {
		t.Fatalf("attach relay client: %v", err)
	}

	tests := []struct {
		source string
		want   []string
	}{
		{source: "relay", want: []string{"juliet@x"}},
		{source: "inbound", want: []string{"alpha@x", "bravo@x", "charlie@x", "delta@x", "echo@x", "foxtrot@x", "golf@x", "hotel@x", "india@x", "kilo1@x"}},
		{source: "standalone", want: []string{standalone.Email}},
	}
	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{PageSize: 50, Source: tc.source})
			if err != nil {
				t.Fatalf("ListPaged: %v", err)
			}
			if got := pagedEmails(resp.Items); !slices.Equal(got, tc.want) {
				t.Fatalf("emails = %v, want %v", got, tc.want)
			}
			if resp.Filtered != len(tc.want) {
				t.Fatalf("filtered = %d, want %d", resp.Filtered, len(tc.want))
			}
		})
	}
}

func TestListPagedSorting(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	seedPagingClients(t)

	tests := []struct {
		name  string
		sort  string
		order string
		want  []string
	}{
		{
			name: "no sort key keeps insertion order",
			want: []string{"alpha@x", "bravo@x", "charlie@x"},
		},
		{
			name: "email ascending", sort: "email", order: "ascend",
			want: []string{"alpha@x", "bravo@x", "charlie@x"},
		},
		{
			name: "email descending", sort: "email", order: "descend",
			want: []string{"kilo_1@x", "kilo1@x", "juliet@x"},
		},
		{
			name: "traffic descending", sort: "traffic", order: "descend",
			want: []string{"charlie@x", "hotel@x", "alpha@x"},
		},
		{
			name: "remaining descending puts unlimited quotas first", sort: "remaining", order: "descend",
			want: []string{"alpha@x", "india@x", "juliet@x"},
		},
		{
			name: "expiry ascending starts with the expired", sort: "expiryTime", order: "ascend",
			want: []string{"delta@x", "foxtrot@x", "golf@x"},
		},
		{
			name: "createdAt ascending follows insertion", sort: "createdAt", order: "ascend",
			want: []string{"alpha@x", "bravo@x", "charlie@x"},
		},
		{
			name: "updatedAt descending starts with the newest", sort: "updatedAt", order: "descend",
			want: []string{"alpha@x", "bravo@x", "charlie@x"},
		},
		{
			name: "lastOnline descending breaks ties on the id, reversed too", sort: "lastOnline", order: "descend",
			want: []string{"bravo@x", "alpha@x", "kilo1@x"},
		},
		{
			name: "enable ascending puts disabled first", sort: "enable", order: "ascend",
			want: []string{"echo@x", "foxtrot@x", "alpha@x"},
		},
		{
			name: "inboundIds descending puts the widest attachment first", sort: "inboundIds", order: "descend",
			want: []string{"bravo@x", "alpha@x", "charlie@x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{PageSize: 3, Sort: tc.sort, Order: tc.order})
			if err != nil {
				t.Fatalf("ListPaged: %v", err)
			}
			got := pagedEmails(resp.Items)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("emails = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListPagedPagination(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	seedPagingClients(t)

	t.Run("second page continues where the first stopped", func(t *testing.T) {
		resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{Page: 2, PageSize: 5, Sort: "email", Order: "ascend"})
		if err != nil {
			t.Fatalf("ListPaged: %v", err)
		}
		want := []string{"foxtrot@x", "golf@x", "hotel@x", "india@x", "juliet@x"}
		if got := pagedEmails(resp.Items); !slices.Equal(got, want) {
			t.Fatalf("emails = %v, want %v", got, want)
		}
		if resp.Page != 2 || resp.PageSize != 5 {
			t.Fatalf("page/pageSize = %d/%d, want 2/5", resp.Page, resp.PageSize)
		}
	})

	t.Run("page past the end is empty but keeps the counts", func(t *testing.T) {
		resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{Page: 9, PageSize: 5})
		if err != nil {
			t.Fatalf("ListPaged: %v", err)
		}
		if len(resp.Items) != 0 {
			t.Fatalf("items = %v, want none", pagedEmails(resp.Items))
		}
		if resp.Filtered != 12 || resp.Total != 12 {
			t.Fatalf("filtered/total = %d/%d, want 12/12", resp.Filtered, resp.Total)
		}
	})

	t.Run("page size is clamped to the maximum", func(t *testing.T) {
		resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{Page: 1, PageSize: 5000})
		if err != nil {
			t.Fatalf("ListPaged: %v", err)
		}
		if resp.PageSize != clientPageMaxSize {
			t.Fatalf("pageSize = %d, want %d", resp.PageSize, clientPageMaxSize)
		}
	})
}

func TestListPagedRowContents(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	seedPagingClients(t)

	resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{PageSize: 50})
	if err != nil {
		t.Fatalf("ListPaged: %v", err)
	}
	byEmail := make(map[string]ClientSlim, len(resp.Items))
	for _, it := range resp.Items {
		byEmail[it.Email] = it
	}

	t.Run("attachments are reported in inbound order", func(t *testing.T) {
		got := byEmail["bravo@x"].InboundIds
		if !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("inboundIds = %v, want [1 2]", got)
		}
	})

	t.Run("an unattached client reports no inbounds", func(t *testing.T) {
		if got := byEmail["kilo_1@x"].InboundIds; len(got) != 0 {
			t.Fatalf("inboundIds = %v, want none", got)
		}
	})

	t.Run("traffic counters ride along with the row", func(t *testing.T) {
		got := byEmail["hotel@x"].Traffic
		if got == nil {
			t.Fatal("traffic = nil, want the seeded counters")
		}
		if want := 10*pagingGB - pagingGB/2; got.Up+got.Down != want {
			t.Fatalf("used = %d, want %d", got.Up+got.Down, want)
		}
	})

	t.Run("groups list every name in use", func(t *testing.T) {
		if !slices.Equal(resp.Groups, []string{"vip", "VIP"}) && !slices.Equal(resp.Groups, []string{"VIP", "vip"}) {
			t.Fatalf("groups = %v, want VIP and vip", resp.Groups)
		}
	})
}

func TestListPagedSummary(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	seedPagingClients(t)

	resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{PageSize: 5, Filter: "depleted"})
	if err != nil {
		t.Fatalf("ListPaged: %v", err)
	}
	s := resp.Summary

	t.Run("counts stay whole-panel while the page is filtered", func(t *testing.T) {
		if s.Total != 12 {
			t.Fatalf("total = %d, want 12", s.Total)
		}
		if s.DepletedCount != 3 {
			t.Fatalf("depletedCount = %d, want 3", s.DepletedCount)
		}
		if s.ExpiringCount != 2 {
			t.Fatalf("expiringCount = %d, want 2", s.ExpiringCount)
		}
		if s.DeactiveCount != 1 {
			t.Fatalf("deactiveCount = %d, want 1", s.DeactiveCount)
		}
		if s.Active != 6 {
			t.Fatalf("active = %d, want 6", s.Active)
		}
	})

	t.Run("every client lands in exactly one counter", func(t *testing.T) {
		if sum := s.Active + s.DepletedCount + s.ExpiringCount + s.DeactiveCount; sum != s.Total {
			t.Fatalf("buckets sum to %d, want %d", sum, s.Total)
		}
	})

	t.Run("bucket lists carry the matching emails", func(t *testing.T) {
		if want := []string{"charlie@x", "delta@x", "foxtrot@x"}; !slices.Equal(s.Depleted, want) {
			t.Fatalf("depleted = %v, want %v", s.Depleted, want)
		}
		if want := []string{"golf@x", "hotel@x"}; !slices.Equal(s.Expiring, want) {
			t.Fatalf("expiring = %v, want %v", s.Expiring, want)
		}
		if want := []string{"echo@x"}; !slices.Equal(s.Deactive, want) {
			t.Fatalf("deactive = %v, want %v", s.Deactive, want)
		}
	})
}

func TestListPagedSummaryEmailListsAreCapped(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	db := database.GetDB()

	const n = clientSummaryEmailCap + 25
	past := time.Now().UnixMilli() - pagingDay
	for i := range n {
		rec := model.ClientRecord{Email: "bulk-" + strconv.Itoa(i) + "@x", Enable: true, TotalGB: pagingGB, ExpiryTime: past}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("create client %d: %v", i, err)
		}
	}

	resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{PageSize: 25})
	if err != nil {
		t.Fatalf("ListPaged: %v", err)
	}
	if resp.Summary.DepletedCount != n {
		t.Fatalf("depletedCount = %d, want %d", resp.Summary.DepletedCount, n)
	}
	if len(resp.Summary.Depleted) != clientSummaryEmailCap {
		t.Fatalf("depleted list = %d entries, want %d", len(resp.Summary.Depleted), clientSummaryEmailCap)
	}
}

func TestListPagedGlobalTrafficOverlay(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	seedPagingClients(t)

	// bravo has used 1GB of its 10GB locally; a master reporting 10GB of
	// cross-panel usage has to move it into the depleted bucket.
	if err := inboundSvc.AcceptGlobalTraffic("master-guid", []*xray.ClientTraffic{
		{Email: "bravo@x", Up: 4 * pagingGB, Down: 6 * pagingGB},
	}); err != nil {
		t.Fatalf("AcceptGlobalTraffic: %v", err)
	}

	resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{PageSize: 50, Filter: "depleted"})
	if err != nil {
		t.Fatalf("ListPaged: %v", err)
	}
	want := []string{"bravo@x", "charlie@x", "delta@x", "foxtrot@x"}
	if got := pagedEmails(resp.Items); !slices.Equal(got, want) {
		t.Fatalf("depleted = %v, want %v", got, want)
	}
	if resp.Summary.DepletedCount != 4 {
		t.Fatalf("depletedCount = %d, want 4", resp.Summary.DepletedCount)
	}
	for _, it := range resp.Items {
		if it.Email != "bravo@x" {
			continue
		}
		if it.Traffic == nil || it.Traffic.Up+it.Traffic.Down != 10*pagingGB {
			t.Fatalf("bravo traffic = %+v, want the overlaid 10GB", it.Traffic)
		}
	}
}

func TestClientQueryOnlineEmails(t *testing.T) {
	_, _, _ = setupPagingServices(t)
	seedPagingClients(t)

	q := newClientQuery(database.GetDB(), time.Now().UnixMilli(), 0, 0)
	emails, count, err := q.onlineEmails([]string{"alpha@x", "echo@x", "ghost@x", "kilo1@x"})
	if err != nil {
		t.Fatalf("onlineEmails: %v", err)
	}
	if want := []string{"alpha@x", "kilo1@x"}; !slices.Equal(emails, want) {
		t.Fatalf("online = %v, want %v (disabled and unknown emails drop out)", emails, want)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestEmailInCondChunksLargeSets(t *testing.T) {
	emails := make([]string, sqlInChunk+1)
	for i := range emails {
		emails[i] = "e" + strconv.Itoa(i)
	}

	cond, args := emailInCond("c.email", emails)
	if want := "(c.email IN ? OR c.email IN ?)"; cond != want {
		t.Fatalf("cond = %q, want %q", cond, want)
	}
	if len(args) != 2 {
		t.Fatalf("args = %d chunks, want 2", len(args))
	}
	if first, ok := args[0].([]string); !ok || len(first) != sqlInChunk {
		t.Fatalf("first chunk = %v, want %d entries", args[0], sqlInChunk)
	}

	emptyCond, emptyArgs := emailInCond("c.email", nil)
	if emptyCond != "1 = 0" || emptyArgs != nil {
		t.Fatalf("empty set = %q/%v, want an always-false predicate", emptyCond, emptyArgs)
	}
}

func TestEscapeLikeLiteral(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"a_b", `a\_b`},
		{"50%", `50\%`},
		{`back\slash`, `back\\slash`},
	}
	for _, tc := range tests {
		if got := escapeLikeLiteral(tc.in); got != tc.want {
			t.Fatalf("escapeLikeLiteral(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestListPagedEmptyPanel(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)

	resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{})
	if err != nil {
		t.Fatalf("ListPaged on a panel with no clients: %v", err)
	}
	if len(resp.Items) != 0 || resp.Total != 0 || resp.Filtered != 0 {
		t.Fatalf("items/total/filtered = %d/%d/%d, want 0/0/0", len(resp.Items), resp.Total, resp.Filtered)
	}
	if resp.Summary.Active != 0 || resp.Summary.DepletedCount != 0 {
		t.Fatalf("summary = %+v, want zeroed counters", resp.Summary)
	}
	if resp.Groups == nil {
		t.Fatal("groups = nil, want an empty list so the filter drawer renders")
	}
}
