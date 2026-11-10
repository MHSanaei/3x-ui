package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// filterOutboundsRejectedByCore drops outbounds the vendored xray-core config
// loader refuses to build — since v26.7.11 that includes unencrypted
// vless/trojan outbounds to public addresses — because one such outbound in
// the merged config would keep the whole core from starting. When the running
// core predates that rejection, unencrypted outbounds are kept, mirroring
// CheckXrayConfig's version gate.
func filterOutboundsRejectedByCore(label string, outbounds []any) ([]any, []string) {
	coreVersion := "Unknown"
	if process := currentXrayProcess(); process != nil {
		coreVersion = process.GetXrayVersion()
	}
	kept := make([]any, 0, len(outbounds))
	var dropped []string
	for _, ob := range outbounds {
		raw, err := json.Marshal(ob)
		if err == nil {
			if buildErr := xray.ValidateOutboundConfig(raw); buildErr != nil && !shouldSkipLegacyUnencryptedOutboundRejection(coreVersion, raw, buildErr) {
				tag := ""
				if m, ok := ob.(map[string]any); ok {
					tag, _ = m["tag"].(string)
				}
				logger.Warningf("%s: dropping outbound %q rejected by xray-core: %v", label, tag, buildErr)
				dropped = append(dropped, fmt.Sprintf("%s: %v", tag, buildErr))
				continue
			}
		}
		kept = append(kept, ob)
	}
	return kept, dropped
}

// maxOutboundSubscriptionBytes caps a single outbound subscription response.
// It is larger than the 2 MiB user-facing subscription cap because an outbound
// subscription may aggregate many upstream outbounds into one document.
const maxOutboundSubscriptionBytes int64 = 8 << 20

const defaultOutboundSubscriptionUserAgent = "3x-ui-outbound-sub/1.0"

var errOutboundSubscriptionBodyTooLarge = errors.New("outbound subscription response body exceeds size limit")

func readBoundedOutboundSubscriptionBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxOutboundSubscriptionBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxOutboundSubscriptionBytes {
		return nil, fmt.Errorf("%w (limit: %d bytes)", errOutboundSubscriptionBodyTooLarge, maxOutboundSubscriptionBytes)
	}
	return body, nil
}

// OutboundSubscriptionService manages remote outbound subscriptions.
type OutboundSubscriptionService struct {
	settingService SettingService
}

// NewOutboundSubscriptionService returns a service for managing outbound subscriptions.
func NewOutboundSubscriptionService() *OutboundSubscriptionService {
	return &OutboundSubscriptionService{}
}

// List returns all subscriptions (newest first).
func (s *OutboundSubscriptionService) List() ([]*model.OutboundSubscription, error) {
	db := database.GetDB()
	var subs []*model.OutboundSubscription
	if err := db.Model(&model.OutboundSubscription{}).Order("priority asc, id asc").Find(&subs).Error; err != nil {
		return nil, err
	}
	for _, sub := range subs {
		sub.OutboundCount = countOutbounds(sub.LastFetchedOutbounds)
		// Don't ship the heavy raw blobs to the list view.
		sub.LastFetchedOutbounds = ""
		sub.LinkIdentities = ""
	}
	return subs, nil
}

// countOutbounds returns the number of outbounds in a stored LastFetchedOutbounds
// JSON array (0 for empty/invalid).
func countOutbounds(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	var arr []any
	if json.Unmarshal([]byte(raw), &arr) != nil {
		return 0
	}
	return len(arr)
}

// Get returns a single subscription by id.
func (s *OutboundSubscriptionService) Get(id int) (*model.OutboundSubscription, error) {
	db := database.GetDB()
	var sub model.OutboundSubscription
	if err := db.First(&sub, id).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

// Create persists a new subscription. It does not fetch immediately; the caller
// can call Refresh on the returned id if desired.
var defaultPrefixRe = regexp.MustCompile(`^sub(\d+)-$`)

// defaultPrefixNumber returns the smallest positive integer N that is not already
// in use as a "subN-" tag prefix among the given subscriptions. This is used to
// auto-name a subscription's outbounds when the user leaves the prefix blank, so
// deleting a subscription frees its number for reuse instead of letting the
// number grow forever with the auto-increment DB id. A subscription with a blank
// prefix reserves its own id (it falls back to id-based "sub<id>-" tags).
func defaultPrefixNumber(subs []*model.OutboundSubscription, excludeId int) int {
	used := map[int]bool{}
	for _, sub := range subs {
		if sub.Id == excludeId {
			continue
		}
		if sub.TagPrefix == "" {
			used[sub.Id] = true
			continue
		}
		if m := defaultPrefixRe.FindStringSubmatch(sub.TagPrefix); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				used[n] = true
			}
		}
	}
	n := 1
	for used[n] {
		n++
	}
	return n
}

// nextDefaultSubPrefix builds the default "subN-" prefix for a new/edited
// subscription, picking the smallest free N (excludeId skips a subscription's
// own current prefix when editing).
func (s *OutboundSubscriptionService) nextDefaultSubPrefix(excludeId int) (string, error) {
	var subs []*model.OutboundSubscription
	if err := database.GetDB().Find(&subs).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("sub%d-", defaultPrefixNumber(subs, excludeId)), nil
}

func (s *OutboundSubscriptionService) Create(remark, rawURL, tagPrefix, userAgent string, enabled bool, updateInterval int, allowPrivate, prepend, allowInsecure bool) (*model.OutboundSubscription, error) {
	cleanURL, err := SanitizePublicHTTPURL(rawURL, allowPrivate)
	if err != nil {
		return nil, common.NewError("invalid subscription URL:", WithoutRequestURL(err))
	}
	if cleanURL == "" {
		return nil, common.NewError("subscription URL is required")
	}
	if updateInterval <= 0 {
		updateInterval = 600
	}
	prefix := strings.TrimSpace(tagPrefix)
	if prefix == "" {
		prefix, err = s.nextDefaultSubPrefix(0)
		if err != nil {
			return nil, err
		}
	}
	// New subscriptions go to the end of the priority order.
	var count int64
	if err := database.GetDB().Model(&model.OutboundSubscription{}).Count(&count).Error; err != nil {
		return nil, err
	}
	sub := &model.OutboundSubscription{
		Remark:         strings.TrimSpace(remark),
		Url:            cleanURL,
		Enabled:        enabled,
		AllowPrivate:   allowPrivate,
		AllowInsecure:  allowInsecure,
		UserAgent:      strings.TrimSpace(userAgent),
		Prepend:        prepend,
		Priority:       int(count),
		TagPrefix:      prefix,
		UpdateInterval: updateInterval,
	}
	if err := database.GetDB().Create(sub).Error; err != nil {
		return nil, err
	}
	// GORM leaves a zero value out of the INSERT, and the column defaults to enabled.
	if !enabled {
		if err := database.GetDB().Model(sub).Update("enabled", false).Error; err != nil {
			return nil, err
		}
	}
	return sub, nil
}

// normalizedSubscriptionPrefix returns the canonical "prefix-" string that
// assignStableTags / link.SuggestTag produce for a given subscription id and tagPrefix.
func normalizedSubscriptionPrefix(subID int, tagPrefix string) string {
	prefix := strings.TrimSpace(tagPrefix)
	if prefix == "" {
		prefix = fmt.Sprintf("sub%d-", subID)
	}
	if p := strings.TrimSuffix(prefix, "-"); p != "" {
		return p + "-"
	}
	return prefix
}

// Update updates editable fields.
func (s *OutboundSubscriptionService) Update(id int, remark, rawURL, tagPrefix, userAgent string, enabled bool, updateInterval int, allowPrivate, prepend, allowInsecure bool) error {
	sub, err := s.Get(id)
	if err != nil {
		return err
	}
	cleanURL, err := SanitizePublicHTTPURL(rawURL, allowPrivate)
	if err != nil {
		return common.NewError("invalid subscription URL:", WithoutRequestURL(err))
	}
	if cleanURL == "" {
		return common.NewError("subscription URL is required")
	}
	if updateInterval <= 0 {
		updateInterval = 600
	}
	prefix := strings.TrimSpace(tagPrefix)
	if prefix == "" && sub.TagPrefix != "" {
		prefix, err = s.nextDefaultSubPrefix(sub.Id)
		if err != nil {
			return err
		}
	}
	oldEffective := normalizedSubscriptionPrefix(sub.Id, sub.TagPrefix)
	newEffective := normalizedSubscriptionPrefix(sub.Id, prefix)

	// Only the editable columns: a whole-row save would put back fetch results that a
	// refresh stored meanwhile, or re-create a row deleted meanwhile.
	updates := map[string]any{
		"remark":          strings.TrimSpace(remark),
		"url":             cleanURL,
		"enabled":         enabled,
		"allow_private":   allowPrivate,
		"allow_insecure":  allowInsecure,
		"user_agent":      strings.TrimSpace(userAgent),
		"prepend":         prepend,
		"tag_prefix":      prefix,
		"update_interval": updateInterval,
	}
	if oldEffective != newEffective {
		s.rewriteStoredPrefix(sub, oldEffective, newEffective, updates)
	}
	return database.GetDB().Model(&model.OutboundSubscription{}).Where("id = ?", sub.Id).Updates(updates).Error
}

// rewriteStoredPrefix updates persisted LinkIdentities and LastFetchedOutbounds
// in-place when an operator changes a subscription's TagPrefix, so that panel UI
// surfaces and subsequent refreshes (including shortened prefixes like "eu-de-" -> "eu-")
// immediately reflect the new prefix without keeping stale prefix segments.
func (s *OutboundSubscriptionService) rewriteStoredPrefix(sub *model.OutboundSubscription, oldPrefix, newPrefix string, updates map[string]any) {
	if strings.TrimSpace(sub.LinkIdentities) == "" && strings.TrimSpace(sub.LastFetchedOutbounds) == "" {
		return
	}
	externalUsed := s.reservedExternalTags(sub.Id)
	used := make(map[string]bool)
	renamed := make(map[string]string)

	allocate := func(oldTag string) string {
		if oldTag == "" {
			return ""
		}
		if mapped, ok := renamed[oldTag]; ok {
			return mapped
		}
		base := oldTag
		if strings.HasPrefix(oldTag, oldPrefix) {
			rest := strings.TrimPrefix(oldTag, oldPrefix)
			if rest != "" {
				base = newPrefix + rest
			}
		}
		final := base
		for k := 1; used[final] || externalUsed[final]; k++ {
			final = fmt.Sprintf("%s-%d", base, k)
		}
		used[final] = true
		renamed[oldTag] = final
		return final
	}

	if strings.TrimSpace(sub.LastFetchedOutbounds) != "" {
		var obs []any
		if json.Unmarshal([]byte(sub.LastFetchedOutbounds), &obs) == nil {
			for _, item := range obs {
				if m, ok := item.(map[string]any); ok {
					if t, _ := m["tag"].(string); t != "" {
						m["tag"] = allocate(t)
					}
				}
			}
			if b, err := json.Marshal(obs); err == nil {
				updates["last_fetched_outbounds"] = string(b)
			}
		}
	}
	if strings.TrimSpace(sub.LinkIdentities) != "" {
		var prev map[string]string
		if json.Unmarshal([]byte(sub.LinkIdentities), &prev) == nil {
			for id, t := range prev {
				if t != "" {
					prev[id] = allocate(t)
				}
			}
			if b, err := json.Marshal(prev); err == nil {
				updates["link_identities"] = string(b)
			}
		}
	}
}

// Delete removes a subscription.
func (s *OutboundSubscriptionService) Delete(id int) error {
	return database.GetDB().Delete(&model.OutboundSubscription{}, id).Error
}

// GetLastOutbounds returns the last successfully fetched outbounds for a subscription
// (as raw interface slice ready for JSON merge). Returns nil slice when none.
func (s *OutboundSubscriptionService) GetLastOutbounds(id int) ([]any, error) {
	sub, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sub.LastFetchedOutbounds) == "" {
		return nil, nil
	}
	var arr []any
	if err := json.Unmarshal([]byte(sub.LastFetchedOutbounds), &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

// Refresh fetches the subscription URL, parses the links, assigns stable tags,
// persists the results, and returns the generated outbounds.
func (s *OutboundSubscriptionService) Refresh(id int) ([]any, error) {
	sub, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	outbounds, err := s.fetchAndStore(sub)
	return outbounds, err
}

// RefreshAllEnabled fetches every enabled subscription whose due time has passed
// (lastUpdated + updateInterval <= now). It returns the number of subscriptions
// that were actually refreshed.
func (s *OutboundSubscriptionService) RefreshAllEnabled() (int, error) {
	db := database.GetDB()
	var subs []*model.OutboundSubscription
	if err := db.Where("enabled = ?", true).Find(&subs).Error; err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	refreshed := 0
	for _, sub := range subs {
		due := sub.LastUpdated + int64(sub.UpdateInterval)
		if sub.LastUpdated == 0 || due <= now {
			if _, err := s.fetchAndStore(sub); err != nil {
				logger.Warningf("outbound sub %d (%s) refresh failed: %v", sub.Id, sub.Remark, err)
				// continue with others
			} else {
				refreshed++
			}
		}
	}
	return refreshed, nil
}

// subscriptionFetchClient builds the HTTP client used to fetch a subscription.
// A configured panel egress proxy dials the loopback SOCKS bridge (xray handles
// the real egress), so its localhost dial must not be SSRF-blocked. A direct
// fetch dials the target itself and re-resolves the hostname at dial time, so it
// goes through the SSRF-guarded dialer, which resolves, checks and dials the same
// IP atomically — closing the DNS-rebinding gap left by validating the hostname
// separately from the dial.
func (s *OutboundSubscriptionService) subscriptionFetchClient(timeout time.Duration, allowInsecure bool) *http.Client {
	var client *http.Client
	if s.settingService.PanelEgressProxyURL() != "" {
		client = s.settingService.NewProxiedHTTPClient(timeout)
	} else {
		client = &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{DialContext: netsafe.SSRFGuardedDialContext},
		}
	}
	if allowInsecure {
		if tr, ok := client.Transport.(*http.Transport); ok && tr != nil {
			cloned := tr.Clone()
			if cloned.TLSClientConfig == nil {
				cloned.TLSClientConfig = &tls.Config{}
			}
			cloned.TLSClientConfig.InsecureSkipVerify = true
			client.Transport = cloned
		}
	}
	return client
}

// subscriptionFetch is what one download of a subscription produces, before it is stored.
type subscriptionFetch struct {
	url        string
	outbounds  []any
	identities string
	lastError  string
}

// fetchAndStore downloads a subscription and stores the result on its row.
func (s *OutboundSubscriptionService) fetchAndStore(sub *model.OutboundSubscription) ([]any, error) {
	readURL := sub.Url
	fetched, err := s.fetchOutbounds(sub)
	if err != nil {
		s.recordError(sub, err)
		return nil, err
	}
	obsJSON, _ := json.Marshal(fetched.outbounds)
	// Only the fetch's own columns, and only while the row still points where it was read:
	// the download takes as long as the provider likes, and the admin may edit or delete meanwhile.
	res := database.GetDB().Model(&model.OutboundSubscription{}).
		Where("id = ? AND url = ?", sub.Id, readURL).
		Updates(map[string]any{
			"url":                    fetched.url,
			"last_fetched_outbounds": string(obsJSON),
			"link_identities":        fetched.identities,
			"last_updated":           time.Now().Unix(),
			"last_error":             common.TruncateStatusText(fetched.lastError),
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, common.NewError("subscription was deleted or pointed at another URL during the fetch")
	}
	return fetched.outbounds, nil
}

// Preview downloads and parses a subscription URL without storing anything.
func (s *OutboundSubscriptionService) Preview(rawURL, userAgent string, allowPrivate, allowInsecure bool) ([]any, error) {
	prefix, err := s.nextDefaultSubPrefix(0)
	if err != nil {
		return nil, err
	}
	fetched, err := s.fetchOutbounds(&model.OutboundSubscription{
		Url: rawURL, UserAgent: userAgent, AllowPrivate: allowPrivate, AllowInsecure: allowInsecure, TagPrefix: prefix,
	})
	if err != nil {
		return nil, err
	}
	return fetched.outbounds, nil
}

// fetchOutbounds does the network, parse, filter and tag-stability work for sub.
func (s *OutboundSubscriptionService) fetchOutbounds(sub *model.OutboundSubscription) (*subscriptionFetch, error) {
	// Re-sanitize on every fetch (handles legacy rows + defense in depth against
	// any direct DB tampering). Private targets are blocked unless this
	// subscription was explicitly created with AllowPrivate.
	cleanURL, err := SanitizePublicHTTPURL(sub.Url, sub.AllowPrivate)
	if err != nil {
		return nil, WithoutRequestURL(err)
	}
	if cleanURL == "" {
		return nil, common.NewError("subscription has no valid URL")
	}

	client := s.subscriptionFetchClient(30*time.Second, sub.AllowInsecure)
	// The transport lives for this fetch only; left alone, the provider decides how long
	// the connections it parks stay open.
	defer client.CloseIdleConnections()
	// Re-validate every redirect hop: the initial host is checked above, but a
	// redirect could still point at a private/internal address (SSRF). Cap the
	// redirect chain as well.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if sub.AllowPrivate {
			return nil
		}
		ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
		defer cancel()
		return rejectPrivateHost(ctx, req.URL.Hostname())
	}

	reqCtx := netsafe.ContextWithAllowPrivate(context.Background(), sub.AllowPrivate)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, cleanURL, nil)
	if err != nil {
		return nil, WithoutRequestURL(err)
	}
	userAgent := strings.TrimSpace(sub.UserAgent)
	if userAgent == "" {
		userAgent = defaultOutboundSubscriptionUserAgent
	}
	req.Header.Set("User-Agent", userAgent)
	// A 3x-ui donor with an HWID limit answers 404 when the header is empty (#6574).
	if hwid := ExternalSubscriptionHwid(); hwid != "" {
		req.Header.Set("X-HWID", hwid)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, WithoutRequestURL(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := readBoundedOutboundSubscriptionBody(resp.Body)
	if err != nil {
		return nil, err
	}

	parsed, identities, err := link.ParseSubscriptionBody(body)
	if err != nil {
		return nil, err
	}

	// Load previous identities -> tags for stability
	prev := map[string]string{}
	if strings.TrimSpace(sub.LinkIdentities) != "" {
		_ = json.Unmarshal([]byte(sub.LinkIdentities), &prev)
	}

	// Also load previous outbounds so we can reuse tags even for identities we
	// temporarily lost (defensive).
	prevTagByIndex := map[int]string{}
	if strings.TrimSpace(sub.LastFetchedOutbounds) != "" {
		var prevObs []any
		if json.Unmarshal([]byte(sub.LastFetchedOutbounds), &prevObs) == nil {
			for i, o := range prevObs {
				if m, ok := o.(map[string]any); ok {
					if tag, _ := m["tag"].(string); tag != "" {
						prevTagByIndex[i] = tag
					}
				}
			}
		}
	}

	// Drop refused and core-rejected links before tagging: prevTagByIndex indexes the
	// persisted (filtered) list, so positions must be counted in that same list.
	var dropped []string
	ownPorts := xrayOwnPorts()
	xmcBudget := maxSubscriptionXMCMasks
	keptLinks, keptIdentities := parsed[:0], identities[:0]
	for i, ob := range parsed {
		reason := internalDialTarget(map[string]any(ob), sub.AllowPrivate, ownPorts)
		if reason == "" && !spendXMCBudget(map[string]any(ob), &xmcBudget) {
			reason = fmt.Sprintf("the subscription uses more than %d xmc masks", maxSubscriptionXMCMasks)
		}
		if reason != "" {
			logger.Warningf("outbound sub %d: dropping outbound %.64q: %s", sub.Id, ob["tag"], reason)
			dropped = append(dropped, fmt.Sprintf("%.64s: %s", ob["tag"], reason))
			continue
		}
		forceUserspaceWireguard(map[string]any(ob))
		if _, rejected := filterOutboundsRejectedByCore(fmt.Sprintf("outbound sub %d", sub.Id), []any{map[string]any(ob)}); len(rejected) > 0 {
			dropped = append(dropped, rejected...)
			continue
		}
		keptLinks = append(keptLinks, ob)
		keptIdentities = append(keptIdentities, identities[i])
	}

	// Assign tags with stability (identity reuse, positional fallback, then a
	// fresh allocation), keeping tags unique within this batch and across other
	// existing subscriptions / template outbounds.
	var externalUsed map[string]bool
	if sub.Id > 0 {
		externalUsed = s.reservedExternalTags(sub.Id)
	}
	assigned := assignStableTagsWithExternal(keptLinks, keptIdentities, prev, prevTagByIndex, sub.Id, sub.TagPrefix, externalUsed)

	// Persist identities for next time
	newIdent := map[string]string{}
	for i, id := range keptIdentities {
		newIdent[id] = assigned[i]
	}
	identJSON, _ := json.Marshal(newIdent)

	kept := make([]any, len(keptLinks))
	for i := range keptLinks {
		kept[i] = map[string]any(keptLinks[i])
	}

	fetched := &subscriptionFetch{url: cleanURL, outbounds: kept, identities: string(identJSON)}
	if len(dropped) > 0 {
		fetched.lastError = fmt.Sprintf("dropped %d outbound(s): %s", len(dropped), dropped[0])
	}
	if len(parsed) >= link.MaxSubscriptionOutbounds {
		fetched.lastError = strings.TrimPrefix(fetched.lastError+"; ", "; ") +
			fmt.Sprintf("only the first %d links of the document are loaded", link.MaxSubscriptionOutbounds)
	}
	return fetched, nil
}

// maxSubscriptionXMCMasks bounds the RSA keys one subscription costs the core per build;
// the masks can be spread over links, so a per-stream cap alone does not bound it.
const maxSubscriptionXMCMasks = 64

func spendXMCBudget(outbound any, budget *int) bool {
	ob, ok := outbound.(map[string]any)
	if !ok {
		return true
	}
	stream, err := json.Marshal(ob["streamSettings"])
	if err != nil {
		return true
	}
	n := xray.CountXMCMasks(stream)
	if n > *budget {
		return false
	}
	*budget -= n
	return true
}

// WithoutRequestURL keeps only the host of the URL net/url and net/http quote in their
// errors: a subscription URL is a credential, and these errors are logged and stored.
func WithoutRequestURL(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	if u, parseErr := url.Parse(urlErr.URL); parseErr == nil && u.Host != "" {
		return fmt.Errorf("%s %s: %w", urlErr.Op, u.Host, urlErr.Err)
	}
	return fmt.Errorf("%s: %w", urlErr.Op, urlErr.Err)
}

// forceUserspaceWireguard keeps a provider's wireguard fields off the host: the kernel
// TUN path would install its addresses and routes on the panel's machine.
func forceUserspaceWireguard(outbound any) {
	ob, ok := outbound.(map[string]any)
	if !ok {
		return
	}
	if protocol, _ := ob["protocol"].(string); !strings.EqualFold(protocol, "wireguard") {
		return
	}
	if settings, ok := ob["settings"].(map[string]any); ok {
		settings["noKernelTun"] = true
	} else {
		ob["settings"] = map[string]any{"noKernelTun": true}
	}
}

func (s *OutboundSubscriptionService) recordError(sub *model.OutboundSubscription, err error) {
	sub.LastError = common.TruncateStatusText(err.Error())
	_ = database.GetDB().Model(sub).Update("last_error", sub.LastError).Error
}

// assignStableTags assigns a tag to each parsed outbound, preferring stability:
//  1. reuse the tag previously mapped to the link's identity (prev),
//  2. else reuse the tag at the same position from the last fetch (prevTagByIndex),
//  3. else allocate a fresh tag from the prefix + remark (link.SuggestTag).
//
// Tags are kept unique within the batch by appending "-N" on collision, and are
// written back into parsed[i]["tag"]. The returned slice holds the assigned tags
// in order. When tagPrefix is empty a "sub<subID>-" prefix is used for fresh tags.
var trailingCollisionNumRe = regexp.MustCompile(`^(.+)-\d+$`)

// matchesActiveTagPrefix reports whether oldTag belongs to expectedPrefix rather
// than a different or longer prefix before a prefix edit (such as shortening
// "eu-de-" to "eu-", where "eu-de-frankfurt" still starts with "eu-" but carries
// the extra "de-" segment ahead of the link's own slug "frankfurt").
func matchesActiveTagPrefix(oldTag, expectedPrefix, baseSlug string) bool {
	if !strings.HasPrefix(oldTag, expectedPrefix) {
		return false
	}
	rest := strings.TrimPrefix(oldTag, expectedPrefix)
	if rest == "" {
		return false
	}
	if baseSlug != "" {
		if rest != baseSlug && strings.HasSuffix(rest, "-"+baseSlug) {
			return false
		}
		if m := trailingCollisionNumRe.FindStringSubmatch(rest); m != nil {
			if m[1] != baseSlug && strings.HasSuffix(m[1], "-"+baseSlug) {
				return false
			}
		}
	}
	return true
}

func linkBaseSlug(ob link.Outbound, idx int) string {
	remark := ""
	if m, ok := ob["tag"].(string); ok {
		remark = m
	}
	base := link.SlugRemark(remark)
	if base == "" {
		base = fmt.Sprintf("%d", idx)
	}
	return base
}

func assignStableTags(parsed []link.Outbound, identities []string, prev map[string]string, prevTagByIndex map[int]string, subID int, tagPrefix string) []string {
	return assignStableTagsWithExternal(parsed, identities, prev, prevTagByIndex, subID, tagPrefix, nil)
}

func assignStableTagsWithExternal(parsed []link.Outbound, identities []string, prev map[string]string, prevTagByIndex map[int]string, subID int, tagPrefix string, externalUsed map[string]bool) []string {
	prefix := tagPrefix
	if prefix == "" {
		prefix = fmt.Sprintf("sub%d-", subID)
	}
	expectedPrefix := normalizedSubscriptionPrefix(subID, prefix)

	reservedStableTags := map[string]bool{}
	for i := range parsed {
		if i < len(identities) && prev[identities[i]] != "" {
			tag := prev[identities[i]]
			if matchesActiveTagPrefix(tag, expectedPrefix, linkBaseSlug(parsed[i], i)) && !externalUsed[tag] {
				reservedStableTags[tag] = true
			}
		}
	}

	used := map[string]bool{} // uniqueness within this refresh batch
	assigned := make([]string, len(parsed))
	for i := range parsed {
		id := ""
		if i < len(identities) {
			id = identities[i]
		}
		baseSlug := linkBaseSlug(parsed[i], i)
		candidate := ""
		identityTag := ""
		if old, ok := prev[id]; ok && old != "" && matchesActiveTagPrefix(old, expectedPrefix, baseSlug) {
			candidate = old
			identityTag = old
		}
		if candidate == "" {
			// try to reuse by rough positional match from previous fetch (best effort)
			if old, ok := prevTagByIndex[i]; ok && old != "" && matchesActiveTagPrefix(old, expectedPrefix, baseSlug) && !reservedStableTags[old] {
				candidate = old
			}
		}
		if candidate == "" {
			// fresh allocation
			remark := ""
			if m, ok := parsed[i]["tag"].(string); ok {
				remark = m
			}
			candidate = link.SuggestTag(prefix, remark, i)
		}
		// ensure uniqueness inside this batch and against template / other subscriptions
		final := candidate
		for k := 1; used[final] || externalUsed[final] || (reservedStableTags[final] && final != identityTag); k++ {
			final = fmt.Sprintf("%s-%d", candidate, k)
		}
		used[final] = true
		assigned[i] = final

		// write back the tag into the outbound
		parsed[i]["tag"] = final
	}
	return assigned
}

// templateOutboundTags returns the set of tags defined in the stored Xray template's
// manual outbounds array.
func (s *OutboundSubscriptionService) templateOutboundTags() map[string]bool {
	used := make(map[string]bool)
	if database.GetDB() == nil {
		return used
	}
	tpl, err := s.settingService.GetXrayConfigTemplate()
	if err != nil || strings.TrimSpace(tpl) == "" {
		return used
	}
	var cfg xray.Config
	if err := json.Unmarshal([]byte(tpl), &cfg); err != nil || len(cfg.OutboundConfigs) == 0 {
		return used
	}
	var templateOutbounds []any
	if err := json.Unmarshal(cfg.OutboundConfigs, &templateOutbounds); err != nil {
		return used
	}
	for _, item := range templateOutbounds {
		if ob, ok := item.(map[string]any); ok {
			if tag, _ := ob["tag"].(string); tag != "" {
				used[tag] = true
			}
		}
	}
	return used
}

// reservedExternalTags returns all outbound tags currently reserved by the Xray
// template and by other subscriptions (id != excludeSubID), so a refresh or prefix
// update can persist collision-free tags directly in LastFetchedOutbounds.
func (s *OutboundSubscriptionService) reservedExternalTags(excludeSubID int) map[string]bool {
	used := s.templateOutboundTags()
	db := database.GetDB()
	if db == nil {
		return used
	}
	var subs []*model.OutboundSubscription
	if err := db.Where("id <> ?", excludeSubID).Order("priority asc, id asc").Find(&subs).Error; err != nil {
		return used
	}
	for _, sub := range subs {
		if strings.TrimSpace(sub.LastFetchedOutbounds) == "" {
			continue
		}
		var arr []any
		if json.Unmarshal([]byte(sub.LastFetchedOutbounds), &arr) != nil {
			continue
		}
		for _, item := range arr {
			if ob, ok := item.(map[string]any); ok {
				if tag, _ := ob["tag"].(string); tag != "" {
					used[tag] = true
				}
			}
		}
	}
	return used
}

// dedupeSubscriptionOutboundList ensures every outbound in list has a tag not in
// usedTags, cloning and appending a "-N" suffix (with a warning log) on collision.
func dedupeSubscriptionOutboundList(list []any, usedTags map[string]bool) []any {
	out := make([]any, len(list))
	for i, item := range list {
		ob, ok := item.(map[string]any)
		if !ok {
			out[i] = item
			continue
		}
		tag, _ := ob["tag"].(string)
		if tag == "" {
			out[i] = item
			continue
		}
		if !usedTags[tag] {
			usedTags[tag] = true
			out[i] = item
			continue
		}
		cloned := make(map[string]any, len(ob))
		for k, v := range ob {
			cloned[k] = v
		}
		final := tag
		for k := 1; usedTags[final]; k++ {
			final = fmt.Sprintf("%s-%d", tag, k)
		}
		usedTags[final] = true
		cloned["tag"] = final
		logger.Warningf("outbound subscription tag %q collides with an existing outbound, renamed to %q", tag, final)
		out[i] = cloned
	}
	return out
}

// AllActiveOutbounds returns the concatenation of the last-fetched outbounds
// for every enabled subscription. This is the set that should be merged into
// the final Xray config.
func (s *OutboundSubscriptionService) AllActiveOutbounds() ([]any, error) {
	prepend, appendList, err := s.activeOutboundsSplit()
	if err != nil {
		return nil, err
	}
	return append(prepend, appendList...), nil
}

// activeOutboundsSplit returns the active subscription outbounds split into those
// that should be placed BEFORE the manual template outbounds (Prepend) and those
// placed AFTER. Within each group, subscriptions are ordered by Priority (then id)
// so the admin can control the merged order. Any colliding tags across template
// and subscriptions are disambiguated consistently so that AllActiveOutbounds,
// AllActiveOutboundTags, and the generated Xray config always expose identical tags.
func (s *OutboundSubscriptionService) activeOutboundsSplit() (prepend []any, appendList []any, err error) {
	db := database.GetDB()
	var subs []*model.OutboundSubscription
	if err := db.Where("enabled = ?", true).Order("priority asc, id asc").Find(&subs).Error; err != nil {
		return nil, nil, err
	}
	ownPorts := xrayOwnPorts()
	for _, sub := range subs {
		xmcBudget := maxSubscriptionXMCMasks
		if strings.TrimSpace(sub.LastFetchedOutbounds) == "" {
			continue
		}
		var arr []any
		if err := json.Unmarshal([]byte(sub.LastFetchedOutbounds), &arr); err != nil {
			logger.Warningf("outbound sub %d has corrupt LastFetchedOutbounds: %v", sub.Id, err)
			continue
		}
		// Rows stored before the fetch refused these, or while allowPrivate was on.
		arr = slices.DeleteFunc(arr, func(ob any) bool {
			reason := internalDialTarget(ob, sub.AllowPrivate, ownPorts)
			if reason == "" && !spendXMCBudget(ob, &xmcBudget) {
				reason = "over the subscription's xmc mask budget"
			}
			if reason != "" {
				logger.Warningf("outbound sub %d: dropping a stored outbound: %s", sub.Id, reason)
			}
			return reason != ""
		})
		for _, ob := range arr {
			forceUserspaceWireguard(ob)
		}
		arr, _ = filterOutboundsRejectedByCore(fmt.Sprintf("outbound sub %d", sub.Id), arr)
		if sub.Prepend {
			prepend = append(prepend, arr...)
		} else {
			appendList = append(appendList, arr...)
		}
	}
	usedTags := s.templateOutboundTags()
	prepend = dedupeSubscriptionOutboundList(prepend, usedTags)
	appendList = dedupeSubscriptionOutboundList(appendList, usedTags)
	return prepend, appendList, nil
}

// Move shifts a subscription one step up or down in the priority order and
// re-normalizes all priorities to a 0..n-1 sequence.
func (s *OutboundSubscriptionService) Move(id int, up bool) error {
	db := database.GetDB()
	var subs []*model.OutboundSubscription
	if err := db.Order("priority asc, id asc").Find(&subs).Error; err != nil {
		return err
	}
	idx := -1
	for i, sub := range subs {
		if sub.Id == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return common.NewError("subscription not found")
	}
	swap := idx + 1
	if up {
		swap = idx - 1
	}
	if swap < 0 || swap >= len(subs) {
		return nil // already at the edge
	}
	subs[idx], subs[swap] = subs[swap], subs[idx]
	for i, sub := range subs {
		if sub.Priority != i {
			if err := db.Model(sub).Update("priority", i).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// AllActiveOutboundTags returns only the tags of active subscription outbounds.
// Useful for populating balancer / routing selectors without shipping full objects.
func (s *OutboundSubscriptionService) AllActiveOutboundTags() ([]string, error) {
	obs, err := s.AllActiveOutbounds()
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(obs))
	for _, o := range obs {
		if m, ok := o.(map[string]any); ok {
			if t, _ := m["tag"].(string); t != "" {
				tags = append(tags, t)
			}
		}
	}
	return tags, nil
}

/*
Tag stability strategy (important for balancers and routing rules)

When a subscription is refreshed we try very hard to keep the *same* tag for the
same logical outbound so that existing balancers and routing rules keep working.

How we do it:
- On every successful parse we compute a stable "identity" for each link
  (the core of the URI with the remark fragment removed, or for vmess the inner
  JSON without the "ps" field).
- We persist a map identity -> tag in the LinkIdentities column.
- On the next refresh, if we see the same identity again we reuse the previous tag,
  even if the remark changed or minor parameters moved.
- Only when we have never seen the identity before do we allocate a fresh tag
  using the user-supplied TagPrefix + slug(remark) (or an index fallback).
- Within one refresh we still deduplicate with -N suffixes.

Consequences for balancers / routing:
- If you use an *exact* tag in a balancer selector or a routing rule, that
  specific server will continue to be used after refreshes (as long as the
  provider still returns a link that produces the same identity).
- If you use a *prefix/wildcard* selector (e.g. "hk-*", "sg-.*"), then any
  *new* servers that the subscription later returns will automatically be
  eligible for that balancer on the next Xray reload — this is the recommended
  way to "subscribe to a pool".
- When a server disappears from the subscription, its tag simply stops
  existing in the final outbounds array. The balancer will have fewer
  candidates. If you configured a `fallbackTag` on the balancer, Xray will use
  it. Otherwise connections that would have used the missing member may fail
  or be routed by the next rule.
- If the provider rotates credentials/UUIDs/hosts for a server, the identity
  changes → we treat it as a brand new outbound and give it a new tag. Any
  balancer/rule that referenced the *old* tag will no longer see it. This is
  an inherent limitation of subscription-based outbounds.

We deliberately do *not* mutate the saved xrayTemplateConfig. Subscription
outbounds are always injected at runtime in GetXrayConfig.
*/

// xrayOwnPorts lists the ports of Xray's gRPC API and metrics listeners, which ask
// for no credentials: being reachable from loopback only is their whole protection.
func xrayOwnPorts() []string {
	ports := []string{strconv.Itoa(reservedAPIPort())}
	metrics, _ := (&XrayMetricsService{}).discoverListen()
	if _, port, err := net.SplitHostPort(metrics); err == nil {
		ports = append(ports, port)
	}
	return ports
}

// internalDialTarget says why a subscription outbound must not be loaded, or "". Keys
// match at any depth (xhttp downloadSettings is a second server) and case-folded like the core.
func internalDialTarget(outbound any, allowPrivate bool, ownPorts []string) string {
	switch v := outbound.(type) {
	case map[string]any:
		for key, child := range v {
			text, isText := child.(string)
			switch {
			case strings.EqualFold(key, "port"):
				// A public name may resolve to loopback by dial time, so the port alone decides.
				if slices.Contains(ownPorts, fmt.Sprint(child)) {
					return "its port belongs to Xray's own API or metrics listener"
				}
			case strings.EqualFold(key, "addressPortStrategy"):
				if text != "" && !strings.EqualFold(text, "none") {
					return "addressPortStrategy takes its server from DNS records"
				}
			case strings.EqualFold(key, "address"), strings.EqualFold(key, "endpoint"):
				// WireGuard's settings.address is a list of tunnel addresses, never dialled.
				if !isText {
					break
				}
				if _, port, err := net.SplitHostPort(strings.TrimSpace(text)); err == nil && slices.Contains(ownPorts, port) {
					return "its port belongs to Xray's own API or metrics listener"
				}
				if !allowPrivate && namesInternalHost(text) {
					return fmt.Sprintf("it connects to the private/internal address %.64q and allowPrivate is off", text)
				}
			}
			if reason := internalDialTarget(child, allowPrivate, ownPorts); reason != "" {
				return reason
			}
		}
	case []any:
		for _, child := range v {
			if reason := internalDialTarget(child, allowPrivate, ownPorts); reason != "" {
				return reason
			}
		}
	}
	return ""
}

// namesInternalHost reports whether host, with or without a port, names this host or
// its network once xray-core and Go's dialer have read it.
func namesInternalHost(host string) bool {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[] ")
	host, _, _ = strings.Cut(host, "%")
	if ip := net.ParseIP(host); ip != nil {
		return isBlockedIP(ip)
	}
	// An empty host is the local system and a name without a dot comes from
	// /etc/hosts or the search domain ("env:" has none either).
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return !strings.Contains(host, ".") || host == "localhost" || strings.HasSuffix(host, ".localhost")
}
