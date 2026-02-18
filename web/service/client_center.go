package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/random"
	"github.com/mhsanaei/3x-ui/v2/xray"
	"gorm.io/gorm"
)

// ClientCenterService provides client-first management on top of inbound-scoped clients.
// It stores master client profiles and synchronizes assigned inbound clients safely.
type ClientCenterService struct {
	inboundService InboundService
}

type ClientCenterInboundInfo struct {
	Id       int    `json:"id"`
	Remark   string `json:"remark"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Enable   bool   `json:"enable"`
}

type MasterClientView struct {
	Id               int                       `json:"id"`
	Name             string                    `json:"name"`
	EmailPrefix      string                    `json:"emailPrefix"`
	TotalGB          int64                     `json:"totalGB"`
	ExpiryTime       int64                     `json:"expiryTime"`
	LimitIP          int                       `json:"limitIp"`
	Enable           bool                      `json:"enable"`
	Comment          string                    `json:"comment"`
	Assignments      []ClientCenterInboundInfo `json:"assignments"`
	UsageUp          int64                     `json:"usageUp"`
	UsageDown        int64                     `json:"usageDown"`
	UsageAllTime     int64                     `json:"usageAllTime"`
	LastSeenOnlineAt int64                     `json:"lastSeenOnlineAt"`
}

type UpsertMasterClientInput struct {
	Name        string
	EmailPrefix string
	TotalGB     int64
	ExpiryTime  int64
	LimitIP     int
	Enable      bool
	Comment     string
	InboundIds  []int
}

func (s *ClientCenterService) ListInbounds(userId int) ([]ClientCenterInboundInfo, error) {
	inbounds, err := s.inboundService.GetInbounds(userId)
	if err != nil {
		return nil, err
	}
	out := make([]ClientCenterInboundInfo, 0, len(inbounds))
	for _, inbound := range inbounds {
		if !supportsManagedClients(inbound.Protocol) {
			continue
		}
		out = append(out, ClientCenterInboundInfo{
			Id:       inbound.Id,
			Remark:   inbound.Remark,
			Protocol: string(inbound.Protocol),
			Port:     inbound.Port,
			Enable:   inbound.Enable,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out, nil
}

func (s *ClientCenterService) ListMasterClients(userId int) ([]MasterClientView, error) {
	db := database.GetDB()
	masters := make([]model.MasterClient, 0)
	if err := db.Where("user_id = ?", userId).Order("id asc").Find(&masters).Error; err != nil {
		return nil, err
	}
	if len(masters) == 0 {
		return []MasterClientView{}, nil
	}

	masterIDs := make([]int, 0, len(masters))
	for _, m := range masters {
		masterIDs = append(masterIDs, m.Id)
	}

	links := make([]model.MasterClientInbound, 0)
	if err := db.Where("master_client_id IN ?", masterIDs).Order("id asc").Find(&links).Error; err != nil {
		return nil, err
	}

	inbounds, err := s.inboundService.GetInbounds(userId)
	if err != nil {
		return nil, err
	}
	inboundByID := map[int]*model.Inbound{}
	for _, inbound := range inbounds {
		inboundByID[inbound.Id] = inbound
	}

	emails := make([]string, 0, len(links))
	for _, l := range links {
		emails = append(emails, l.AssignmentEmail)
	}
	trafficByEmail := map[string]xray.ClientTraffic{}
	if len(emails) > 0 {
		stats := make([]xray.ClientTraffic, 0)
		if err := db.Where("email IN ?", emails).Find(&stats).Error; err == nil {
			for _, st := range stats {
				trafficByEmail[strings.ToLower(st.Email)] = st
			}
		}
	}

	linksByMaster := map[int][]model.MasterClientInbound{}
	for _, l := range links {
		linksByMaster[l.MasterClientId] = append(linksByMaster[l.MasterClientId], l)
	}

	result := make([]MasterClientView, 0, len(masters))
	for _, m := range masters {
		view := MasterClientView{
			Id:          m.Id,
			Name:        m.Name,
			EmailPrefix: m.EmailPrefix,
			TotalGB:     m.TotalGB,
			ExpiryTime:  m.ExpiryTime,
			LimitIP:     m.LimitIP,
			Enable:      m.Enable,
			Comment:     m.Comment,
		}
		for _, link := range linksByMaster[m.Id] {
			if inbound, ok := inboundByID[link.InboundId]; ok {
				view.Assignments = append(view.Assignments, ClientCenterInboundInfo{
					Id:       inbound.Id,
					Remark:   inbound.Remark,
					Protocol: string(inbound.Protocol),
					Port:     inbound.Port,
					Enable:   inbound.Enable,
				})
			}
			if st, ok := trafficByEmail[strings.ToLower(link.AssignmentEmail)]; ok {
				view.UsageUp += st.Up
				view.UsageDown += st.Down
				view.UsageAllTime += st.AllTime
				if st.LastOnline > view.LastSeenOnlineAt {
					view.LastSeenOnlineAt = st.LastOnline
				}
			}
		}
		sort.Slice(view.Assignments, func(i, j int) bool { return view.Assignments[i].Id < view.Assignments[j].Id })
		result = append(result, view)
	}
	return result, nil
}

func (s *ClientCenterService) CreateMasterClient(userId int, input UpsertMasterClientInput) (*model.MasterClient, error) {
	normalized, err := normalizeMasterInput(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	master := &model.MasterClient{
		UserId:      userId,
		Name:        normalized.Name,
		EmailPrefix: normalized.EmailPrefix,
		TotalGB:     normalized.TotalGB,
		ExpiryTime:  normalized.ExpiryTime,
		LimitIP:     normalized.LimitIP,
		Enable:      normalized.Enable,
		Comment:     normalized.Comment,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	db := database.GetDB()
	if err := db.Create(master).Error; err != nil {
		return nil, err
	}
	if err := s.syncMasterAssignments(userId, master, normalized.InboundIds); err != nil {
		return nil, err
	}
	return master, nil
}

func (s *ClientCenterService) UpdateMasterClient(userId, masterID int, input UpsertMasterClientInput) (*model.MasterClient, error) {
	normalized, err := normalizeMasterInput(input)
	if err != nil {
		return nil, err
	}
	db := database.GetDB()
	master := &model.MasterClient{}
	if err := db.Where("id = ? AND user_id = ?", masterID, userId).First(master).Error; err != nil {
		return nil, err
	}
	master.Name = normalized.Name
	master.EmailPrefix = normalized.EmailPrefix
	master.TotalGB = normalized.TotalGB
	master.ExpiryTime = normalized.ExpiryTime
	master.LimitIP = normalized.LimitIP
	master.Enable = normalized.Enable
	master.Comment = normalized.Comment
	master.UpdatedAt = time.Now().UnixMilli()
	if err := db.Save(master).Error; err != nil {
		return nil, err
	}
	if err := s.syncMasterAssignments(userId, master, normalized.InboundIds); err != nil {
		return nil, err
	}
	return master, nil
}

func (s *ClientCenterService) DeleteMasterClient(userId, masterID int) error {
	db := database.GetDB()
	master := &model.MasterClient{}
	if err := db.Where("id = ? AND user_id = ?", masterID, userId).First(master).Error; err != nil {
		return err
	}
	links := make([]model.MasterClientInbound, 0)
	if err := db.Where("master_client_id = ?", masterID).Find(&links).Error; err != nil {
		return err
	}

	for _, link := range links {
		if err := s.removeAssignment(link); err != nil {
			return err
		}
	}
	if err := db.Where("master_client_id = ?", masterID).Delete(&model.MasterClientInbound{}).Error; err != nil {
		return err
	}
	return db.Delete(master).Error
}

func (s *ClientCenterService) syncMasterAssignments(userId int, master *model.MasterClient, desiredInboundIDs []int) error {
	db := database.GetDB()
	links := make([]model.MasterClientInbound, 0)
	if err := db.Where("master_client_id = ?", master.Id).Find(&links).Error; err != nil {
		return err
	}

	desired := map[int]bool{}
	for _, id := range desiredInboundIDs {
		desired[id] = true
	}
	existing := map[int]model.MasterClientInbound{}
	for _, l := range links {
		existing[l.InboundId] = l
	}

	inbounds, err := s.inboundService.GetInbounds(userId)
	if err != nil {
		return err
	}
	inboundByID := map[int]*model.Inbound{}
	for _, inbound := range inbounds {
		inboundByID[inbound.Id] = inbound
	}

	for inboundID := range desired {
		inbound, ok := inboundByID[inboundID]
		if !ok {
			return common.NewError("inbound not found for user:", inboundID)
		}
		if !supportsManagedClients(inbound.Protocol) {
			return common.NewError("inbound protocol is not multi-client:", inbound.Protocol)
		}
		if link, exists := existing[inboundID]; exists {
			if err := s.updateAssignment(master, inbound, link); err != nil {
				return err
			}
			continue
		}
		if err := s.createAssignment(master, inboundID, inbound); err != nil {
			return err
		}
	}

	for inboundID, link := range existing {
		if desired[inboundID] {
			continue
		}
		if err := s.removeAssignment(link); err != nil {
			return err
		}
		if err := db.Delete(&link).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *ClientCenterService) createAssignment(master *model.MasterClient, inboundID int, inbound *model.Inbound) error {
	db := database.GetDB()
	assignEmail := s.newAssignmentEmail(master.EmailPrefix, inboundID)
	client, clientKey := buildProtocolClient(master, assignEmail, inbound.Protocol)

	payload := map[string]any{"clients": []model.Client{client}}
	settingsJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data := &model.Inbound{Id: inboundID, Settings: string(settingsJSON)}
	if _, err := s.inboundService.AddInboundClient(data); err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	link := &model.MasterClientInbound{
		MasterClientId:  master.Id,
		InboundId:       inboundID,
		AssignmentEmail: assignEmail,
		ClientKey:       clientKey,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return db.Create(link).Error
}

func (s *ClientCenterService) updateAssignment(master *model.MasterClient, inbound *model.Inbound, link model.MasterClientInbound) error {
	client, _ := buildProtocolClient(master, link.AssignmentEmail, inbound.Protocol)
	payload := map[string]any{"clients": []model.Client{client}}
	settingsJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data := &model.Inbound{Id: inbound.Id, Settings: string(settingsJSON)}
	if _, err := s.inboundService.UpdateInboundClient(data, link.ClientKey); err != nil {
		return err
	}
	link.UpdatedAt = time.Now().UnixMilli()
	return database.GetDB().Save(&link).Error
}

func (s *ClientCenterService) removeAssignment(link model.MasterClientInbound) error {
	_, err := s.inboundService.DelInboundClient(link.InboundId, link.ClientKey)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "no client remained") {
		return common.NewError("cannot detach from inbound because it would leave inbound without clients")
	}
	return err
}

func supportsManagedClients(protocol model.Protocol) bool {
	switch protocol {
	case model.VMESS, model.VLESS, model.Trojan, model.Shadowsocks:
		return true
	default:
		return false
	}
}

func normalizeMasterInput(input UpsertMasterClientInput) (UpsertMasterClientInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.EmailPrefix = strings.TrimSpace(strings.ToLower(input.EmailPrefix))
	input.Comment = strings.TrimSpace(input.Comment)
	if input.Name == "" {
		return input, errors.New("name is required")
	}
	if input.EmailPrefix == "" {
		return input, errors.New("email prefix is required")
	}
	if strings.ContainsAny(input.EmailPrefix, " @") {
		return input, errors.New("email prefix cannot contain spaces or '@'")
	}
	if input.TotalGB < 0 {
		return input, errors.New("totalGB cannot be negative")
	}
	if input.LimitIP < 0 {
		return input, errors.New("limitIp cannot be negative")
	}
	if input.ExpiryTime < 0 {
		return input, errors.New("expiryTime cannot be negative")
	}
	input.InboundIds = dedupeInboundIDs(input.InboundIds)
	return input, nil
}

func dedupeInboundIDs(ids []int) []int {
	set := map[int]bool{}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || set[id] {
			continue
		}
		set[id] = true
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

func (s *ClientCenterService) newAssignmentEmail(prefix string, inboundID int) string {
	base := strings.TrimSpace(strings.ToLower(prefix))
	if base == "" {
		base = "client"
	}
	return fmt.Sprintf("%s.%s.%s@local", base, strconv.Itoa(inboundID), random.Seq(6))
}

func buildProtocolClient(master *model.MasterClient, assignmentEmail string, protocol model.Protocol) (model.Client, string) {
	client := model.Client{
		Email:      assignmentEmail,
		LimitIP:    master.LimitIP,
		TotalGB:    master.TotalGB,
		ExpiryTime: master.ExpiryTime,
		Enable:     master.Enable,
		SubID:      random.Seq(16),
		Comment:    master.Comment,
		Reset:      0,
	}
	switch protocol {
	case model.Trojan:
		client.Password = random.Seq(18)
		return client, client.Password
	case model.Shadowsocks:
		client.Password = random.Seq(18)
		return client, client.Email
	case model.VMESS:
		client.ID = uuid.NewString()
		client.Security = "auto"
		return client, client.ID
	default: // vless and other UUID-based protocols
		client.ID = uuid.NewString()
		return client, client.ID
	}
}

func (s *ClientCenterService) GetMasterClient(userId, masterID int) (*model.MasterClient, error) {
	master := &model.MasterClient{}
	err := database.GetDB().Where("id = ? and user_id = ?", masterID, userId).First(master).Error
	if err != nil {
		return nil, err
	}
	return master, nil
}

func (s *ClientCenterService) EnsureTablesReady() error {
	// No-op helper to keep service extension points explicit.
	return nil
}

func (s *ClientCenterService) IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
