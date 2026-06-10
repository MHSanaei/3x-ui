package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

type CopyClientsResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors"`
}

// enrichClientStats parses each inbound's clients once, fills in the
// UUID/SubId fields on the preloaded ClientStats, and tops up rows owned by
// a sibling inbound (shared-email mode — the row is keyed on email so it
// only preloads on its owning inbound).
func (s *InboundService) enrichClientStats(db *gorm.DB, inbounds []*model.Inbound) {
	if len(inbounds) == 0 {
		return
	}
	clientsByInbound := make([][]model.Client, len(inbounds))
	seenByInbound := make([]map[string]struct{}, len(inbounds))
	missing := make(map[string]struct{})
	for i, inbound := range inbounds {
		clients, _ := s.GetClients(inbound)
		clientsByInbound[i] = clients
		seen := make(map[string]struct{}, len(inbound.ClientStats))
		for _, st := range inbound.ClientStats {
			if st.Email != "" {
				seen[strings.ToLower(st.Email)] = struct{}{}
			}
		}
		seenByInbound[i] = seen
		for _, c := range clients {
			if c.Email == "" {
				continue
			}
			if _, ok := seen[strings.ToLower(c.Email)]; !ok {
				missing[c.Email] = struct{}{}
			}
		}
	}
	if len(missing) > 0 {
		emails := make([]string, 0, len(missing))
		for e := range missing {
			emails = append(emails, e)
		}
		var extra []xray.ClientTraffic
		var loadErr error
		for _, batch := range chunkStrings(emails, sqlInChunk) {
			var page []xray.ClientTraffic
			if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
				loadErr = err
				break
			}
			extra = append(extra, page...)
		}
		if loadErr != nil {
			logger.Warning("enrichClientStats:", loadErr)
		} else {
			byEmail := make(map[string]xray.ClientTraffic, len(extra))
			for _, st := range extra {
				byEmail[strings.ToLower(st.Email)] = st
			}
			for i, inbound := range inbounds {
				for _, c := range clientsByInbound[i] {
					if c.Email == "" {
						continue
					}
					key := strings.ToLower(c.Email)
					if _, ok := seenByInbound[i][key]; ok {
						continue
					}
					if st, ok := byEmail[key]; ok {
						inbound.ClientStats = append(inbound.ClientStats, st)
						seenByInbound[i][key] = struct{}{}
					}
				}
			}
		}
	}
	for i, inbound := range inbounds {
		clients := clientsByInbound[i]
		if len(clients) == 0 || len(inbound.ClientStats) == 0 {
			continue
		}
		cMap := make(map[string]model.Client, len(clients))
		for _, c := range clients {
			cMap[strings.ToLower(c.Email)] = c
		}
		for j := range inbound.ClientStats {
			email := strings.ToLower(inbound.ClientStats[j].Email)
			if c, ok := cMap[email]; ok {
				inbound.ClientStats[j].UUID = c.ID
				inbound.ClientStats[j].SubId = c.SubID
			}
		}
	}
}

// emailUsedByOtherInbounds reports whether email lives in any inbound other
// than exceptInboundId. Empty email returns false.
func (s *InboundService) emailUsedByOtherInbounds(email string, exceptInboundId int) (bool, error) {
	if email == "" {
		return false, nil
	}
	db := database.GetDB()
	var count int64
	query := fmt.Sprintf(
		"SELECT COUNT(*) %s WHERE inbounds.id != ? AND LOWER(%s) = LOWER(?)",
		database.JSONClientsFromInbound(),
		database.JSONFieldText("client.value", "email"),
	)
	if err := db.Raw(query, exceptInboundId, email).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *InboundService) emailsUsedByOtherInbounds(emails []string, exceptInboundId int) (map[string]bool, error) {
	shared := make(map[string]bool, len(emails))
	want := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			want[e] = struct{}{}
		}
	}
	if len(want) == 0 {
		return shared, nil
	}
	db := database.GetDB()
	var rows []string
	query := fmt.Sprintf(
		"SELECT DISTINCT LOWER(%s) %s WHERE inbounds.id != ?",
		database.JSONFieldText("client.value", "email"),
		database.JSONClientsFromInbound(),
	)
	if err := db.Raw(query, exceptInboundId).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, e := range rows {
		e = strings.ToLower(strings.TrimSpace(e))
		if _, ok := want[e]; ok {
			shared[e] = true
		}
	}
	return shared, nil
}

func (s *InboundService) writeBackClientSubID(sourceInboundID int, client model.Client, subID string) (bool, error) {
	client.SubID = subID
	client.UpdatedAt = time.Now().UnixMilli()
	if client.Email == "" {
		return false, common.NewError("empty client email")
	}

	settingsBytes, err := json.Marshal(map[string][]model.Client{
		"clients": {client},
	})
	if err != nil {
		return false, err
	}

	updatePayload := &model.Inbound{
		Id:       sourceInboundID,
		Settings: string(settingsBytes),
	}
	return s.clientService.UpdateInboundClient(s, updatePayload, client.Email)
}

func (s *InboundService) generateRandomCredential(targetProtocol model.Protocol) string {
	switch targetProtocol {
	case model.VMESS, model.VLESS:
		return uuid.NewString()
	default:
		return strings.ReplaceAll(uuid.NewString(), "-", "")
	}
}

func (s *InboundService) buildTargetClientFromSource(source model.Client, targetInbound *model.Inbound, email string, flow string) (model.Client, error) {
	nowTs := time.Now().UnixMilli()
	target := source
	target.Email = email
	target.CreatedAt = nowTs
	target.UpdatedAt = nowTs

	target.ID = ""
	target.Password = ""
	target.Auth = ""
	target.Flow = ""

	targetProtocol := targetInbound.Protocol
	switch targetProtocol {
	case model.VMESS:
		target.ID = s.generateRandomCredential(targetProtocol)
	case model.VLESS:
		target.ID = s.generateRandomCredential(targetProtocol)
		if (flow == "xtls-rprx-vision" || flow == "xtls-rprx-vision-udp443") &&
			inboundCanEnableTlsFlow(string(targetProtocol), targetInbound.StreamSettings) {
			target.Flow = flow
		}
	case model.Trojan, model.Shadowsocks:
		target.Password = s.generateRandomCredential(targetProtocol)
	case model.Hysteria:
		target.Auth = s.generateRandomCredential(targetProtocol)
	default:
		target.ID = s.generateRandomCredential(targetProtocol)
	}

	return target, nil
}

func (s *InboundService) nextAvailableCopiedEmail(originalEmail string, targetID int, occupied map[string]struct{}) string {
	base := fmt.Sprintf("%s_%d", originalEmail, targetID)
	candidate := base
	suffix := 0
	for {
		if _, exists := occupied[strings.ToLower(candidate)]; !exists {
			occupied[strings.ToLower(candidate)] = struct{}{}
			return candidate
		}
		suffix++
		candidate = fmt.Sprintf("%s_%d", base, suffix)
	}
}

func (s *InboundService) CopyInboundClients(targetInboundID int, sourceInboundID int, clientEmails []string, flow string) (*CopyClientsResult, bool, error) {
	result := &CopyClientsResult{
		Added:   []string{},
		Skipped: []string{},
		Errors:  []string{},
	}
	if targetInboundID == sourceInboundID {
		return result, false, common.NewError("source and target inbounds must be different")
	}

	targetInbound, err := s.GetInbound(targetInboundID)
	if err != nil {
		return result, false, err
	}
	sourceInbound, err := s.GetInbound(sourceInboundID)
	if err != nil {
		return result, false, err
	}

	sourceClients, err := s.GetClients(sourceInbound)
	if err != nil {
		return result, false, err
	}
	if len(sourceClients) == 0 {
		return result, false, nil
	}

	allowedEmails := map[string]struct{}{}
	if len(clientEmails) > 0 {
		for _, email := range clientEmails {
			allowedEmails[strings.ToLower(strings.TrimSpace(email))] = struct{}{}
		}
	}

	occupiedEmails := map[string]struct{}{}
	allEmails, err := s.GetAllEmails()
	if err != nil {
		return result, false, err
	}
	for _, email := range allEmails {
		clean := strings.Trim(email, "\"")
		if clean != "" {
			occupiedEmails[strings.ToLower(clean)] = struct{}{}
		}
	}

	newClients := make([]model.Client, 0)
	needRestart := false
	for _, sourceClient := range sourceClients {
		originalEmail := strings.TrimSpace(sourceClient.Email)
		if originalEmail == "" {
			continue
		}
		if len(allowedEmails) > 0 {
			if _, ok := allowedEmails[strings.ToLower(originalEmail)]; !ok {
				continue
			}
		}

		if sourceClient.SubID == "" {
			newSubID := uuid.NewString()
			subNeedRestart, subErr := s.writeBackClientSubID(sourceInbound.Id, sourceClient, newSubID)
			if subErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to write source subId: %v", originalEmail, subErr))
				continue
			}
			if subNeedRestart {
				needRestart = true
			}
			sourceClient.SubID = newSubID
		}

		targetEmail := s.nextAvailableCopiedEmail(originalEmail, targetInboundID, occupiedEmails)
		targetClient, buildErr := s.buildTargetClientFromSource(sourceClient, targetInbound, targetEmail, flow)
		if buildErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", originalEmail, buildErr))
			continue
		}
		newClients = append(newClients, targetClient)
		result.Added = append(result.Added, targetEmail)
	}

	if len(newClients) == 0 {
		return result, needRestart, nil
	}

	settingsPayload, err := json.Marshal(map[string][]model.Client{
		"clients": newClients,
	})
	if err != nil {
		return result, needRestart, err
	}

	addNeedRestart, err := s.clientService.AddInboundClient(s, &model.Inbound{
		Id:       targetInboundID,
		Settings: string(settingsPayload),
	})
	if err != nil {
		return result, needRestart, err
	}
	if addNeedRestart {
		needRestart = true
	}

	return result, needRestart, nil
}

func (s *InboundService) GetClientInboundByTrafficID(trafficId int) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("id = ?", trafficId).Find(&traffics).Error
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with trafficId %d: %v", trafficId, err)
		return nil, nil, err
	}
	if len(traffics) == 0 {
		return nil, nil, nil
	}
	traffic = traffics[0]

	inbound, err = s.GetInbound(traffic.InboundId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// client_traffics.inbound_id goes stale when an inbound is deleted and
		// recreated; fall back to the authoritative client_inbounds link by email.
		ids, idErr := s.clientService.GetInboundIdsForEmail(db, traffic.Email)
		if idErr != nil {
			return traffic, nil, idErr
		}
		if len(ids) > 0 {
			inbound, err = s.GetInbound(ids[0])
		}
	}
	return traffic, inbound, err
}

func (s *InboundService) GetClientInboundByEmail(email string) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics).Error
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, nil, err
	}
	if len(traffics) == 0 {
		return nil, nil, nil
	}
	traffic = traffics[0]

	inbound, err = s.GetInbound(traffic.InboundId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// client_traffics.inbound_id is a legacy single-inbound pointer that goes
		// stale when an inbound is deleted and recreated: the email-keyed traffic
		// row survives but still references the missing inbound. Fall back to the
		// authoritative client_inbounds link so email lookups (reset, info, …) work.
		ids, idErr := s.clientService.GetInboundIdsForEmail(db, email)
		if idErr != nil {
			return traffic, nil, idErr
		}
		if len(ids) > 0 {
			inbound, err = s.GetInbound(ids[0])
		}
	}
	return traffic, inbound, err
}

func (s *InboundService) GetClientByEmail(clientEmail string) (*xray.ClientTraffic, *model.Client, error) {
	traffic, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return nil, nil, err
	}
	if inbound == nil {
		return nil, nil, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return nil, nil, err
	}

	for _, client := range clients {
		if client.Email == clientEmail {
			return traffic, &client, nil
		}
	}

	return nil, nil, common.NewError("Client Not Found In Inbound For Email:", clientEmail)
}

// EmailsByInbound returns the list of client emails currently configured on
// an inbound's settings.clients[]. Used by the "delete all clients" flow on
// the inbounds page, which then feeds the list into ClientService.BulkDelete.
func (s *InboundService) EmailsByInbound(inboundId int) ([]string, error) {
	inbound, err := s.GetInbound(inboundId)
	if err != nil {
		return nil, err
	}
	clients, err := s.GetClients(inbound)
	if err != nil {
		return nil, err
	}
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		if e := strings.TrimSpace(c.Email); e != "" {
			emails = append(emails, e)
		}
	}
	return emails, nil
}
