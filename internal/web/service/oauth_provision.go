package service

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// OAuthProvisionService turns a verified OIDC user-tier identity into a usable
// VPN connection: it guarantees a Client exists on every inbound whose remark
// matches XUI_OAUTH_USER_INBOUND_REMARK.
type OAuthProvisionService struct{}

// EnsureUserClient returns the subId of the caller's client, guaranteeing it is
// attached to every inbound whose remark is listed in XUI_OAUTH_USER_INBOUND_REMARK.
// On first login it creates the client with the env default limits; on later
// logins it reconciles — attaching the existing client (same subId/limits) to any
// newly added matching inbound, so a fresh inbound reaches users without a reset.
func (s *OAuthProvisionService) EnsureUserClient(inboundSvc *InboundService, clientSvc *ClientService, cfg config.OAuthConfig, email string) (string, bool, error) {
	if email == "" {
		return "", false, common.NewError("oauth: cannot provision client without an email")
	}
	if len(cfg.UserInboundRemarks) == 0 {
		return "", false, common.NewError("oauth: XUI_OAUTH_USER_INBOUND_REMARK is not configured")
	}

	inbounds, err := inboundSvc.GetAllInbounds()
	if err != nil {
		return "", false, err
	}
	wanted := make(map[string]struct{}, len(cfg.UserInboundRemarks))
	for _, r := range cfg.UserInboundRemarks {
		wanted[r] = struct{}{}
	}
	var targets []*model.Inbound
	for _, ib := range inbounds {
		if _, ok := wanted[ib.Remark]; ok {
			targets = append(targets, ib)
		}
	}
	if len(targets) == 0 {
		return "", false, common.NewError("oauth: no inbound matches remark(s):", strings.Join(cfg.UserInboundRemarks, ", "))
	}

	existing, presentOn := findClientAcrossTargets(inboundSvc, targets, email)
	var toAttach []int
	for _, ib := range targets {
		if _, ok := presentOn[ib.Id]; !ok {
			toAttach = append(toAttach, ib.Id)
		}
	}
	if len(toAttach) == 0 {
		if existing != nil {
			return existing.SubID, false, nil
		}
		if subID, ok := storedSubIDByEmail(email); ok {
			return subID, false, nil
		}
		return "", false, common.NewError("oauth: client present but subId unresolved for", email)
	}

	client := buildOAuthClient(email, cfg)
	if existing != nil {
		client = *existing
	}
	needRestart, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{
		Client:     client,
		InboundIds: toAttach,
	})
	if err != nil {
		// The client may already exist elsewhere; reuse its subId rather than
		// fail the login.
		if existing != nil {
			return existing.SubID, needRestart, nil
		}
		if subID, ok := storedSubIDByEmail(email); ok {
			return subID, false, nil
		}
		return "", needRestart, err
	}
	return client.SubID, needRestart, nil
}

// ReconcileAll mirrors every user-tier client onto all inbounds matching
// XUI_OAUTH_USER_INBOUND_REMARK, so an inbound added after users logged in reaches
// them without a re-login. It only attaches (never deletes/disables); a client's
// subId, limits and traffic are preserved. Returns how many clients gained an
// inbound and whether xray needs a restart. The remark-matched inbounds are the
// declared user pool, so any client already on one is kept consistent across all.
func (s *OAuthProvisionService) ReconcileAll(inboundSvc *InboundService, clientSvc *ClientService, cfg config.OAuthConfig) (int, bool, error) {
	if len(cfg.UserInboundRemarks) == 0 {
		return 0, false, nil
	}
	inbounds, err := inboundSvc.GetAllInbounds()
	if err != nil {
		return 0, false, err
	}
	wanted := make(map[string]struct{}, len(cfg.UserInboundRemarks))
	for _, r := range cfg.UserInboundRemarks {
		wanted[r] = struct{}{}
	}
	targetIDs := make([]int, 0)
	byEmail := make(map[string]*reconcileEntry)
	for _, ib := range inbounds {
		if _, ok := wanted[ib.Remark]; !ok {
			continue
		}
		targetIDs = append(targetIDs, ib.Id)
		clients, cErr := inboundSvc.GetClients(ib)
		if cErr != nil {
			continue
		}
		for i := range clients {
			email := clients[i].Email
			if email == "" {
				continue
			}
			entry := byEmail[email]
			if entry == nil {
				entry = &reconcileEntry{present: make(map[int]struct{})}
				byEmail[email] = entry
			}
			entry.present[ib.Id] = struct{}{}
			if entry.client.SubID == "" && clients[i].SubID != "" {
				entry.client = clients[i]
			}
		}
	}
	if len(targetIDs) == 0 {
		return 0, false, nil
	}

	attached := 0
	needRestart := false
	for email, entry := range byEmail {
		if entry.client.SubID == "" {
			continue
		}
		var missing []int
		for _, id := range targetIDs {
			if _, ok := entry.present[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			continue
		}
		nr, cErr := clientSvc.Create(inboundSvc, &ClientCreatePayload{Client: entry.client, InboundIds: missing})
		if cErr != nil {
			logger.Warningf("oauth sync: attach %s to %d inbound(s) failed: %v", email, len(missing), cErr)
			continue
		}
		attached++
		if nr {
			needRestart = true
		}
	}
	return attached, needRestart, nil
}

type reconcileEntry struct {
	client  model.Client
	present map[int]struct{}
}

// findClientAcrossTargets locates the caller's client on the target inbounds,
// returning it (first match with a non-empty subId) and the set of inbound ids
// it is already attached to.
func findClientAcrossTargets(inboundSvc *InboundService, targets []*model.Inbound, email string) (*model.Client, map[int]struct{}) {
	presentOn := make(map[int]struct{})
	var found *model.Client
	for _, target := range targets {
		clients, err := inboundSvc.GetClients(target)
		if err != nil {
			continue
		}
		for i := range clients {
			if clients[i].Email == email {
				presentOn[target.Id] = struct{}{}
				if found == nil && clients[i].SubID != "" {
					c := clients[i]
					found = &c
				}
				break
			}
		}
	}
	return found, presentOn
}

func storedSubIDByEmail(email string) (string, bool) {
	record := &model.ClientRecord{}
	err := database.GetDB().Where("email = ?", email).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || err != nil {
		return "", false
	}
	if record.SubID == "" {
		return "", false
	}
	return record.SubID, true
}

// buildOAuthClient mints the client with a known subId so the caller can hand it
// straight to the cabinet; ClientService.Create fills per-protocol credentials.
func buildOAuthClient(email string, cfg config.OAuthConfig) model.Client {
	c := model.Client{
		Email:   email,
		Enable:  true,
		LimitIP: cfg.UserLimitIP,
		TotalGB: cfg.UserTotalGB * 1024 * 1024 * 1024,
		SubID:   uuid.NewString(),
	}
	if cfg.UserExpiryDays > 0 {
		c.ExpiryTime = time.Now().Add(time.Duration(cfg.UserExpiryDays) * 24 * time.Hour).UnixMilli()
	}
	return c
}
