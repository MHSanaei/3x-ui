package sub

import (
	"encoding/base64"
	"net/url"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// externalLinkEntry is one client × external-link row resolved for a request.
// Active applies the owning client's enabled and expiry state; the fetch fields
// travel with the library row the link came from.
type externalLinkEntry struct {
	Kind       string
	Value      string
	Remark     string
	NamePrefix string
	Email      string
	Enable     bool
	Active     bool
	Scope      string
	UserAgent  string
	Headers    map[string]string
	CacheTTL   int
}

// fetchRequest carries the library row's fetch settings into the fetch layer.
func (e externalLinkEntry) fetchRequest() subscriptionRequest {
	return subscriptionRequest{
		URL:       e.Value,
		UserAgent: e.UserAgent,
		Headers:   e.Headers,
		CacheTTL:  time.Duration(e.CacheTTL) * time.Second,
	}
}

// expandedLink is a single share link contributed by an entry, with the display
// name to use (empty → keep the link's own remark / fall back to the email).
type expandedLink struct {
	Link string
	Name string
}

// getClientExternalLinksBySubId resolves every link the clients of this sub id
// receive: their own plus the inherited ones. An inactive owner stays metadata.
func (s *SubService) getClientExternalLinksBySubId(subId string) ([]externalLinkEntry, error) {
	db := database.GetDB()
	var recs []model.ClientRecord
	if err := db.Select("id", "email", "enable", "expiry_time").
		Where("sub_id = ?", subId).Order("id ASC").Find(&recs).Error; err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, nil
	}
	clientIds := make([]int, 0, len(recs))
	for _, rec := range recs {
		clientIds = append(clientIds, rec.Id)
	}
	resolved, err := service.ResolveEffectiveExternalLinks(clientIds)
	if err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	out := []externalLinkEntry{}
	for _, rec := range recs {
		active := rec.Enable && (rec.ExpiryTime <= 0 || rec.ExpiryTime > now)
		for _, link := range resolved[rec.Id] {
			out = append(out, externalLinkEntry{
				Kind:       link.Kind,
				Value:      link.Value,
				Remark:     link.Remark,
				NamePrefix: link.NamePrefix,
				Email:      rec.Email,
				Enable:     rec.Enable,
				Active:     active,
				Scope:      link.Scope,
				UserAgent:  link.UserAgent,
				Headers:    link.Headers,
				CacheTTL:   link.CacheTTL,
			})
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// expandEntry turns one entry into the concrete share links it contributes.
// Names are never blank, so Clash/JSON do not fall back to the client email.
func expandEntry(e externalLinkEntry) []expandedLink {
	if e.Kind == model.ExternalLinkKindSubscription {
		res := fetchSubscriptionLinksFor(e.fetchRequest())
		links := res.links
		if len(links) == 0 {
			// Cold cache during a provider outage: the stored expansion is the
			// only thing standing between the client and an empty subscription.
			links = service.LastExternalLinkLinksByValue(e.Value)
		}
		out := make([]expandedLink, 0, len(links))
		for _, l := range links {
			out = append(out, expandedLink{Link: l, Name: prefixedLinkName(linkDisplayName(l), e.NamePrefix, e.Email)})
		}
		return out
	}
	name := strings.TrimSpace(e.Remark)
	if name == "" {
		name = linkDisplayName(e.Value)
	}
	return []expandedLink{{Link: e.Value, Name: name}}
}

// linkDisplayName extracts the human-readable name already carried by a share
// link: vmess JSON `ps`, or the URL #fragment for every other scheme.
func linkDisplayName(rawLink string) string {
	rawLink = strings.TrimSpace(rawLink)
	if rawLink == "" {
		return ""
	}
	if after, ok := strings.CutPrefix(rawLink, "vmess://"); ok {
		b64 := after
		raw, err := base64.StdEncoding.DecodeString(padBase64Sub(b64))
		if err != nil {
			raw, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(b64, "="))
		}
		if err != nil {
			return ""
		}
		var j map[string]any
		if err := json.Unmarshal(raw, &j); err != nil {
			return ""
		}
		if ps, ok := j["ps"].(string); ok {
			return strings.TrimSpace(ps)
		}
		return ""
	}
	if i := strings.IndexByte(rawLink, '#'); i >= 0 && i+1 < len(rawLink) {
		frag := rawLink[i+1:]
		if decoded, err := url.PathUnescape(frag); err == nil {
			return strings.TrimSpace(decoded)
		}
		return strings.TrimSpace(frag)
	}
	return ""
}

// prefixedLinkName falls back to the client email so a prefixed row never
// renders as the bare prefix when the link carries no name of its own.
func prefixedLinkName(displayName, prefix, fallback string) string {
	if strings.TrimSpace(prefix) == "" {
		return displayName
	}
	name := displayName
	if name == "" {
		name = strings.TrimSpace(fallback)
	}
	return prefix + name
}

// applyRemarkToLink rewrites a share link's display name to remark (when set),
// leaving everything else byte-for-byte. vmess carries its remark in the base64
// JSON `ps`; every other scheme carries it in the URL #fragment.
func applyRemarkToLink(rawLink, remark string) string {
	rawLink = strings.TrimSpace(rawLink)
	if remark == "" {
		return rawLink
	}
	if strings.HasPrefix(rawLink, "vmess://") {
		return applyVmessRemark(rawLink, remark)
	}
	if i := strings.IndexByte(rawLink, '#'); i >= 0 {
		rawLink = rawLink[:i]
	}
	return rawLink + "#" + url.PathEscape(remark)
}

func applyVmessRemark(rawLink, remark string) string {
	b64 := strings.TrimPrefix(rawLink, "vmess://")
	raw, err := base64.StdEncoding.DecodeString(padBase64Sub(b64))
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(b64, "="))
	}
	if err != nil {
		return rawLink
	}
	var j map[string]any
	if err := json.Unmarshal(raw, &j); err != nil {
		return rawLink
	}
	j["ps"] = remark
	nb, err := json.Marshal(j)
	if err != nil {
		return rawLink
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(nb)
}

func padBase64Sub(s string) string {
	for len(s)%4 != 0 {
		s += "="
	}
	return s
}

// parsedExternalOutbound turns a pasted share link into a structured Xray
// outbound (tagged "proxy") for the JSON subscription. Returns nil when the
// link can't be parsed — the caller skips it.
func parsedExternalOutbound(rawLink string) json_util.RawMessage {
	ob := parseExternalLink(rawLink)
	if ob == nil {
		return nil
	}
	ob["tag"] = "proxy"
	b, err := json.MarshalIndent(ob, "", "  ")
	if err != nil {
		return nil
	}
	return b
}

// parseExternalLink parses a share link into the Xray outbound wire shape
// (map), or nil if unsupported/invalid.
func parseExternalLink(rawLink string) map[string]any {
	res, err := link.ParseLink(strings.TrimSpace(rawLink))
	if err != nil || res == nil || res.Outbound == nil {
		return nil
	}
	return map[string]any(res.Outbound)
}
