package sub

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// seedInheritedLink binds one library row to the panel-wide scope, the way the
// library page's assign call does.
func seedInheritedLink(t *testing.T, value string) model.ExternalLink {
	t.Helper()
	link := model.ExternalLink{Kind: model.ExternalLinkKindLink, Value: value, Remark: "panel-wide"}
	if err := database.GetDB().Create(&link).Error; err != nil {
		t.Fatalf("create library link: %v", err)
	}
	assignment := model.ExternalLinkAssignment{
		LinkId:     link.Id,
		TargetType: model.ExternalLinkTargetGlobal,
		Origin:     model.ExternalLinkOriginPanel,
	}
	if err := database.GetDB().Create(&assignment).Error; err != nil {
		t.Fatalf("assign library link: %v", err)
	}
	return link
}

// TestSubscriptionIncludesInheritedPanelWideLink: a link nobody assigned to the
// client directly still reaches the subscription through the panel scope.
func TestSubscriptionIncludesInheritedPanelWideLink(t *testing.T) {
	initSubDB(t)
	const value = "vless://33333333-3333-3333-3333-333333333333@inherited.example:443?type=tcp#inherited"
	seedInheritedLink(t, value)
	rec := &model.ClientRecord{Email: "inherited@x", SubID: "inherited-sub", UUID: "inherited-uuid", Enable: true}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs("inherited-sub", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 || !strings.Contains(links[0], "inherited.example") {
		t.Fatalf("links = %#v, want the panel-wide link", links)
	}
}

// TestExternalSubscriptionSendsPanelIdentityAndRowHeaders pins the outgoing
// request: one stable device (X-HWID = panelGuid), the row's own headers on top.
func TestExternalSubscriptionSendsPanelIdentityAndRowHeaders(t *testing.T) {
	initSubDB(t)
	resetSubscriptionCache(t)
	requests := make(chan http.Header, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Header.Clone()
		_, _ = w.Write([]byte("vless://44444444-4444-4444-4444-444444444444@provider.example:443"))
	}))
	defer srv.Close()

	links := expandEntry(externalLinkEntry{
		Kind:      model.ExternalLinkKindSubscription,
		Value:     srv.URL + "/identity",
		UserAgent: "Happ/2.5.1",
		Headers:   map[string]string{"X-Device-Model": "Pixel 9"},
	})
	if len(links) != 1 {
		t.Fatalf("expanded links = %#v", links)
	}

	var header http.Header
	select {
	case header = <-requests:
	case <-time.After(3 * time.Second):
		t.Fatal("no request reached the provider")
	}
	if got := header.Get("User-Agent"); got != "Happ/2.5.1" {
		t.Fatalf("User-Agent = %q, want the library row's own agent", got)
	}
	if got := header.Get("X-Device-Model"); got != "Pixel 9" {
		t.Fatalf("X-Device-Model = %q, want the library row's custom header", got)
	}
	hwid := header.Get("X-HWID")
	if hwid == "" {
		t.Fatal("X-HWID was not sent")
	}
	// The identity must be stable across fetches, or an HWID-limited donor
	// burns a device slot per request (#6559).
	if stored := serverHwid(); hwid != stored {
		t.Fatalf("X-HWID = %q, want the stored identity %q", hwid, stored)
	}
	if header.Get("X-Device-Os") == "" {
		t.Fatal("X-Device-Os was not sent")
	}
}

// TestSubscriptionServesStoredExpansionWhenTheProviderIsDown: with a cold cache
// and a dead provider, the last good expansion is the only thing between the
// client and an empty subscription after a panel restart.
func TestSubscriptionServesStoredExpansionWhenTheProviderIsDown(t *testing.T) {
	initSubDB(t)
	resetSubscriptionCache(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("vless://44444444-4444-4444-4444-444444444444@live.example:443#live"))
	}))
	providerURL := provider.URL
	provider.Close()

	row := model.ExternalLink{Kind: model.ExternalLinkKindSubscription, Value: providerURL, Remark: "provider"}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatalf("seed library row: %v", err)
	}
	stored := []string{"vless://55555555-5555-5555-5555-555555555555@stored.example:443#stored"}
	encoded, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("encode stored links: %v", err)
	}
	if err := database.GetDB().Model(&model.ExternalLink{}).Where("id = ?", row.Id).
		Update("last_links", string(encoded)).Error; err != nil {
		t.Fatalf("store last expansion: %v", err)
	}
	assignment := model.ExternalLinkAssignment{
		LinkId:     row.Id,
		TargetType: model.ExternalLinkTargetGlobal,
		Origin:     model.ExternalLinkOriginPanel,
	}
	if err := database.GetDB().Create(&assignment).Error; err != nil {
		t.Fatalf("assign library row: %v", err)
	}
	rec := &model.ClientRecord{Email: "stored@x", SubID: "stored-sub", UUID: "stored-uuid", Enable: true}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs("stored-sub", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	if len(links) != 1 || !strings.Contains(links[0], "stored.example") {
		t.Fatalf("links = %#v, want the stored expansion while the provider is unreachable", links)
	}
}

// TestExternalSubscriptionRevalidatesWithValidators: a second fetch of a known
// URL asks the provider whether anything changed and a 304 keeps the body.
func TestExternalSubscriptionRevalidatesWithValidators(t *testing.T) {
	resetSubscriptionCache(t)
	var conditional atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			conditional.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte("vless://55555555-5555-5555-5555-555555555555@cached.example:443"))
	}))
	defer srv.Close()

	req := subscriptionRequest{URL: srv.URL + "/revalidate"}
	first := fetchSubscriptionLinksFor(req)
	if len(first.links) != 1 || !strings.Contains(first.links[0], "cached.example") {
		t.Fatalf("first fetch links = %#v", first.links)
	}

	// Age the entry past its freshness window without making it unservable.
	key := req.cacheKey()
	subscriptionCache.Lock()
	entry := subscriptionCache.m[key]
	entry.refreshAt = time.Now().Add(-time.Second)
	subscriptionCache.m[key] = entry
	subscriptionCache.Unlock()

	refreshed, err := refreshSubscription(req, key)
	if !errors.Is(err, errNotModified) {
		t.Fatalf("refreshSubscription after 304 err = %v, want errNotModified", err)
	}
	if len(refreshed) != 1 || !strings.Contains(refreshed[0], "cached.example") {
		t.Fatalf("revalidated links = %#v, want the cached body kept on 304", refreshed)
	}
	if got := conditional.Load(); got != 1 {
		t.Fatalf("conditional requests = %d, want 1 (If-None-Match must carry the stored ETag)", got)
	}
}

// TestExternalSubscriptionNegativeCacheStopsHammering: a provider answering an
// error is not retried once per client request.
func TestExternalSubscriptionNegativeCacheStopsHammering(t *testing.T) {
	resetSubscriptionCache(t)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	url := srv.URL + "/broken"
	for range 5 {
		if links := fetchSubscriptionLinks(url).links; len(links) != 0 {
			t.Fatalf("failed provider returned links: %#v", links)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1: the negative cache must absorb the rest", got)
	}
}

// TestRefreshExternalSubscriptionBypassesTheCache: the operator's manual
// refresh is the one call that must not be answered from cache.
func TestRefreshExternalSubscriptionBypassesTheCache(t *testing.T) {
	initSubDB(t)
	resetSubscriptionCache(t)
	var body atomic.Value
	body.Store("vless://66666666-6666-6666-6666-666666666666@first.example:443")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body.Load().(string)))
	}))
	defer srv.Close()

	url := srv.URL + "/manual"
	if links := fetchSubscriptionLinks(url).links; len(links) != 1 || !strings.Contains(links[0], "first.example") {
		t.Fatalf("initial fetch links = %#v", links)
	}
	body.Store("vless://77777777-7777-7777-7777-777777777777@second.example:443")

	count, err := RefreshExternalSubscription(url, "", nil, 0)
	if err != nil {
		t.Fatalf("RefreshExternalSubscription: %v", err)
	}
	if count != 1 {
		t.Fatalf("refresh reported %d link(s), want 1", count)
	}
	if links := fetchSubscriptionLinks(url).links; len(links) != 1 || !strings.Contains(links[0], "second.example") {
		t.Fatalf("links after manual refresh = %#v, want the refetched body", links)
	}
}
