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
)

// externalLinkEntry is one client × external-link row, resolved for a
// subscription request. Email/Enable come from the owning client. Active
// applies the owning client's enabled and expiry state.
type externalLinkEntry struct {
	Kind       string
	Value      string
	Remark     string
	NamePrefix string
	Email      string
	Enable     bool
	Active     bool
}

// expandedLink is a single share link contributed by an entry, with the display
// name to use (empty → keep the link's own remark / fall back to the email).
type expandedLink struct {
	Link string
	Name string
}

// getClientExternalLinksBySubId returns every external-link row attached to a
// client that carries the given subId, in stable order. Stays inside
// internal/sub + database + util/link — no dependency on the panel service layer.
func (s *SubService) getClientExternalLinksBySubId(subId string) ([]externalLinkEntry, error) {
	db := database.GetDB()
	var recs []model.ClientRecord
	if err := db.Where("sub_id = ?", subId).Find(&recs).Error; err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, nil
	}
	clientIds := make([]int, 0, len(recs))
	byId := make(map[int]model.ClientRecord, len(recs))
	for _, rec := range recs {
		clientIds = append(clientIds, rec.Id)
		byId[rec.Id] = rec
	}

	var rows []model.ClientExternalLink
	now := time.Now().UnixMilli()
	if err := db.Where("client_id IN ?", clientIds).
		Where("(enable IS NULL OR enable = ?)", true).
		Where("(expiry_time IS NULL OR expiry_time <= 0 OR expiry_time > ?)", now).
		Order("client_id ASC, sort_index ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	out := make([]externalLinkEntry, 0, len(rows))
	for _, r := range rows {
		rec := byId[r.ClientId]
		out = append(out, externalLinkEntry{
			Kind:       r.Kind,
			Value:      r.Value,
			Remark:     r.Remark,
			NamePrefix: r.NamePrefix,
			Email:      rec.Email,
			Enable:     rec.Enable,
			Active:     rec.Enable && (rec.ExpiryTime <= 0 || rec.ExpiryTime > now),
		})
	}
	return out, nil
}

// expandEntry turns one entry into the concrete share links it contributes.
// Names are never blank, so Clash/JSON do not fall back to the client email.
func expandEntry(e externalLinkEntry) []expandedLink {
	if e.Kind == model.ExternalLinkKindSubscription {
		res := fetchSubscriptionLinks(e.Value)
		if res.fetched {
			recordExternalSubscriptionFetch(e.Value, res.err)
		}
		out := make([]expandedLink, 0, len(res.links))
		for _, l := range res.links {
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
