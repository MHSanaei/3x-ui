package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/common"
	"github.com/mhsanaei/3x-ui/v3/util/link"
)

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
	if err := db.Model(&model.OutboundSubscription{}).Order("id desc").Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
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
func (s *OutboundSubscriptionService) Create(remark, rawURL, tagPrefix string, enabled bool, updateInterval int) (*model.OutboundSubscription, error) {
	cleanURL, err := SanitizePublicHTTPURL(rawURL, false)
	if err != nil {
		return nil, common.NewError("invalid subscription URL:", err)
	}
	if cleanURL == "" {
		return nil, common.NewError("subscription URL is required")
	}
	if updateInterval <= 0 {
		updateInterval = 600
	}
	sub := &model.OutboundSubscription{
		Remark:         strings.TrimSpace(remark),
		Url:            cleanURL,
		Enabled:        enabled,
		TagPrefix:      strings.TrimSpace(tagPrefix),
		UpdateInterval: updateInterval,
	}
	if err := database.GetDB().Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

// Update updates editable fields.
func (s *OutboundSubscriptionService) Update(id int, remark, rawURL, tagPrefix string, enabled bool, updateInterval int) error {
	sub, err := s.Get(id)
	if err != nil {
		return err
	}
	cleanURL, err := SanitizePublicHTTPURL(rawURL, false)
	if err != nil {
		return common.NewError("invalid subscription URL:", err)
	}
	if cleanURL == "" {
		return common.NewError("subscription URL is required")
	}
	if updateInterval <= 0 {
		updateInterval = 600
	}
	sub.Remark = strings.TrimSpace(remark)
	sub.Url = cleanURL
	sub.Enabled = enabled
	sub.TagPrefix = strings.TrimSpace(tagPrefix)
	sub.UpdateInterval = updateInterval
	return database.GetDB().Save(sub).Error
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

// fetchAndStore does the actual network + parse + stability + persist work.
func (s *OutboundSubscriptionService) fetchAndStore(sub *model.OutboundSubscription) ([]any, error) {
	// Re-sanitize on every fetch (handles legacy rows + defense in depth against
	// any direct DB tampering). Private targets are blocked.
	cleanURL, err := SanitizePublicHTTPURL(sub.Url, false)
	if err != nil {
		s.recordError(sub, err)
		return nil, err
	}
	if cleanURL == "" {
		return nil, common.NewError("subscription has no valid URL")
	}
	sub.Url = cleanURL // persist the cleaned version

	client := s.settingService.NewProxiedHTTPClient(30 * time.Second)

	req, err := http.NewRequest("GET", sub.Url, nil)
	if err != nil {
		s.recordError(sub, err)
		return nil, err
	}
	req.Header.Set("User-Agent", "3x-ui-outbound-sub/1.0")

	resp, err := client.Do(req)
	if err != nil {
		s.recordError(sub, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err := fmt.Errorf("http %d", resp.StatusCode)
		s.recordError(sub, err)
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.recordError(sub, err)
		return nil, err
	}

	parsed, identities, err := link.ParseSubscriptionBody(body)
	if err != nil {
		s.recordError(sub, err)
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

	// Assign tags with stability
	used := map[string]bool{} // global uniqueness within this refresh
	// Seed used with tags that already exist from *other* subscriptions + template
	// is hard here (we don't have the full picture). We at least avoid collisions
	// inside this subscription's own set, and rely on the caller (config merge)
	// or the user choosing good prefixes. For extra safety we append -N on dup.

	assigned := make([]string, len(parsed))
	for i, id := range identities {
		candidate := ""
		if old, ok := prev[id]; ok && old != "" {
			candidate = old
		}
		if candidate == "" {
			// try to reuse by rough positional match from previous fetch (best effort)
			if old, ok := prevTagByIndex[i]; ok && old != "" {
				candidate = old
			}
		}
		if candidate == "" {
			// fresh allocation
			prefix := sub.TagPrefix
			if prefix == "" {
				prefix = fmt.Sprintf("sub%d-", sub.Id)
			}
			remark := ""
			if m, ok := parsed[i]["tag"].(string); ok {
				remark = m
			}
			candidate = link.SuggestTag(prefix, remark, i)
		}
		// ensure local uniqueness inside this batch
		final := candidate
		for k := 1; used[final]; k++ {
			final = fmt.Sprintf("%s-%d", candidate, k)
		}
		used[final] = true
		assigned[i] = final

		// write back the tag into the outbound
		parsed[i]["tag"] = final
	}

	// Persist identities for next time
	newIdent := map[string]string{}
	for i, id := range identities {
		newIdent[id] = assigned[i]
	}
	identJSON, _ := json.Marshal(newIdent)

	// Persist the outbounds (as compact JSON array)
	obsJSON, _ := json.Marshal(parsed)

	sub.LastFetchedOutbounds = string(obsJSON)
	sub.LinkIdentities = string(identJSON)
	sub.LastUpdated = time.Now().Unix()
	sub.LastError = ""

	if err := database.GetDB().Save(sub).Error; err != nil {
		return nil, err
	}

	// Return as []any for the config merger
	result := make([]any, len(parsed))
	for i := range parsed {
		result[i] = parsed[i]
	}
	return result, nil
}

func (s *OutboundSubscriptionService) recordError(sub *model.OutboundSubscription, err error) {
	sub.LastError = err.Error()
	_ = database.GetDB().Model(sub).Update("last_error", sub.LastError).Error
}

// AllActiveOutbounds returns the concatenation of the last-fetched outbounds
// for every enabled subscription. This is the set that should be merged into
// the final Xray config. Order: subscription creation order (by id asc) so
// that later subscriptions can shadow earlier ones if the admin uses colliding
// prefixes (last writer wins inside xray, but we try to keep tags unique).
func (s *OutboundSubscriptionService) AllActiveOutbounds() ([]any, error) {
	db := database.GetDB()
	var subs []*model.OutboundSubscription
	if err := db.Where("enabled = ?", true).Order("id asc").Find(&subs).Error; err != nil {
		return nil, err
	}
	var all []any
	for _, sub := range subs {
		if strings.TrimSpace(sub.LastFetchedOutbounds) == "" {
			continue
		}
		var arr []any
		if err := json.Unmarshal([]byte(sub.LastFetchedOutbounds), &arr); err != nil {
			logger.Warningf("outbound sub %d has corrupt LastFetchedOutbounds: %v", sub.Id, err)
			continue
		}
		all = append(all, arr...)
	}
	return all, nil
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