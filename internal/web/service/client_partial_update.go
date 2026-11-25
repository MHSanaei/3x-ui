package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// UpdateByEmailJSON applies a POST /clients/update body: an omitted key keeps the
// stored value and an explicit zero, empty or null value clears it.
func (s *ClientService) UpdateByEmailJSON(inboundSvc *InboundService, email string, body []byte, inboundFilter ...int) (bool, error) {
	var req clientPayloadWithHwid
	if err := json.Unmarshal(body, &req); err != nil {
		return false, err
	}
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	keys := clientPayloadKeys(body)
	fillAbsentClientFields(rec, &req.Client, &req.LimitHwid, keys)
	return s.update(inboundSvc, rec.Id, req.Client, req.LimitHwid, !keys.has("flow"), inboundFilter)
}

// payloadKeys is a struct so tools/openapigen, which exports every named
// non-struct type of this package, leaves it out of the generated frontend types.
type payloadKeys struct{ names []string }

func clientPayloadKeys(body []byte) payloadKeys {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return payloadKeys{}
	}
	keys := payloadKeys{names: make([]string, 0, len(raw))}
	for k := range raw {
		keys.names = append(keys.names, k)
	}
	return keys
}

// has matches keys the way encoding/json binds them to fields: case-insensitively,
// so a body sending "limitIP" counts as carrying limitIp.
func (k payloadKeys) has(name string) bool {
	for _, key := range k.names {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

// fillAbsentClientFields leaves flow to update, which keeps it per inbound.
func fillAbsentClientFields(rec *model.ClientRecord, updated *model.Client, limitHwid *int, keys payloadKeys) {
	stored := rec.ToClient()
	if !keys.has("email") {
		updated.Email = stored.Email
	}
	if !keys.has("security") {
		updated.Security = stored.Security
	}
	if !keys.has("limitIp") {
		updated.LimitIP = stored.LimitIP
	}
	if !keys.has("totalGB") {
		updated.TotalGB = stored.TotalGB
	}
	if !keys.has("expiryTime") {
		updated.ExpiryTime = stored.ExpiryTime
	}
	if !keys.has("enable") {
		updated.Enable = stored.Enable
	}
	if !keys.has("tgId") {
		updated.TgID = stored.TgID
	}
	if !keys.has("group") {
		updated.Group = stored.Group
	}
	if !keys.has("comment") {
		updated.Comment = stored.Comment
	}
	if !keys.has("reset") {
		updated.Reset = stored.Reset
	}
	if !keys.has("resetDay") {
		updated.ResetDay = stored.ResetDay
	}
	if !keys.has("resetWeekday") {
		updated.ResetWeekday = stored.ResetWeekday
	}
	if !keys.has("resetMax") {
		updated.ResetMax = stored.ResetMax
	}
	if !keys.has("trafficReset") {
		updated.TrafficReset = stored.TrafficReset
	}
	if !keys.has("trafficResetDay") {
		updated.TrafficResetDay = stored.TrafficResetDay
	}
	if !keys.has("adTag") {
		updated.AdTag = stored.AdTag
	}
	if !keys.has("reverse") {
		updated.Reverse = stored.Reverse
	}
	if !keys.has("limitHwid") {
		*limitHwid = rec.LimitHwid
	}
}

// inboundFlowOverrides returns each linked inbound's flow_override: the reliable
// per-inbound flow, unlike clients.flow (see EffectiveFlow).
func inboundFlowOverrides(recordId int) (map[int]string, error) {
	var links []model.ClientInbound
	if err := database.GetDB().Select("inbound_id", "flow_override").
		Where("client_id = ?", recordId).Find(&links).Error; err != nil {
		return nil, err
	}
	flows := make(map[int]string, len(links))
	for _, link := range links {
		flows[link.InboundId] = link.FlowOverride
	}
	return flows, nil
}
