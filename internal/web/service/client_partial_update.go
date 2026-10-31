package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// still-ui: partial client updates.
//
// POST /panel/api/clients/update/:email binds the body into model.Client, so a
// field the caller left out arrives as its zero value. Before this merge a
// script or bot that sent only {"email":..., "expiryTime":...} silently
// disabled the client (enable=false), dropped its traffic/IP/HWID limits, its
// Telegram id, comment, group, VLESS flow and renewal settings. The panel UI
// always sends the full client, so it is unaffected; API callers now get the
// usual "partial update keeps what it did not mention" semantics. An explicit
// zero ("enable": false, "expiryTime": 0, "comment": "") still clears the field.
//
// Tunnel credentials (WireGuard/AmneziaWG keys, preSharedKey, allowedIPs,
// keepAlive, forwardedPorts) and UUID/password/auth/secret are not handled here:
// they are per-inbound and already carried forward by Update/UpdateInboundClient.

// ClientPayloadKeys returns the set of top-level JSON keys present in an update
// body. A body that is not a JSON object yields an empty set.
func ClientPayloadKeys(body []byte) map[string]bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return map[string]bool{}
	}
	keys := make(map[string]bool, len(raw))
	for k := range raw {
		keys[k] = true
	}
	return keys
}

// FillAbsentClientFields copies every generic client field whose JSON key is
// missing from present out of the stored record for email into updated (and
// limitHwid). Unknown email: nothing is filled; Update reports the error.
func (s *ClientService) FillAbsentClientFields(email string, updated *model.Client, limitHwid *int, present map[string]bool) error {
	email = strings.TrimSpace(email)
	if email == "" || updated == nil {
		return nil
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	FillAbsentClientFieldsFrom(rec, updated, limitHwid, present)
	return nil
}

// FillAbsentClientFieldsFrom is the pure part of FillAbsentClientFields.
func FillAbsentClientFieldsFrom(rec *model.ClientRecord, updated *model.Client, limitHwid *int, present map[string]bool) {
	if rec == nil || updated == nil {
		return
	}
	stored := rec.ToClient()
	absent := func(key string) bool { return !present[key] }

	if absent("email") {
		updated.Email = stored.Email
	}
	if absent("security") {
		updated.Security = stored.Security
	}
	if absent("flow") {
		updated.Flow = stored.Flow
	}
	if absent("reverse") {
		updated.Reverse = stored.Reverse
	}
	if absent("limitIp") {
		updated.LimitIP = stored.LimitIP
	}
	if absent("totalGB") {
		updated.TotalGB = stored.TotalGB
	}
	if absent("expiryTime") {
		updated.ExpiryTime = stored.ExpiryTime
	}
	if absent("enable") {
		updated.Enable = stored.Enable
	}
	if absent("tgId") {
		updated.TgID = stored.TgID
	}
	if absent("group") {
		updated.Group = stored.Group
	}
	if absent("comment") {
		updated.Comment = stored.Comment
	}
	if absent("reset") {
		updated.Reset = stored.Reset
	}
	if absent("resetDay") {
		updated.ResetDay = stored.ResetDay
	}
	if absent("resetWeekday") {
		updated.ResetWeekday = stored.ResetWeekday
	}
	if absent("resetMax") {
		updated.ResetMax = stored.ResetMax
	}
	if absent("trafficReset") {
		updated.TrafficReset = stored.TrafficReset
	}
	if absent("trafficResetDay") {
		updated.TrafficResetDay = stored.TrafficResetDay
	}
	if absent("adTag") {
		updated.AdTag = stored.AdTag
	}
	if limitHwid != nil && absent("limitHwid") {
		*limitHwid = rec.LimitHwid
	}
}
