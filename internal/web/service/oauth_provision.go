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
func (s *OAuthProvisionService) EnsureUserClient(inboundSvc *InboundService, clientSvc *ClientService, cfg config.OAuthConfig, email string) (subID string, needRestart bool, err error) {
	if email == "" {
		return "", false, common.NewError("oauth: cannot provision client without an email")
	}
	if len(cfg.UserInboundRemarks) == 0 {
		return "", false, common.NewError("oauth: XUI_OAUTH_USER_INBOUND_REMARK is not configured")
	}
	// Tag the client so the reconcile job can restore it onto inbounds even after
	// every matching inbound was deleted (its ClientRecord outlives the inbound).
	defer func() {
		if err == nil && subID != "" {
			if markErr := markOauthManaged(email); markErr != nil {
				logger.Warning("oauth: mark managed client failed:", markErr)
			}
		}
	}()

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
	needRestart, err = clientSvc.Create(inboundSvc, &ClientCreatePayload{
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

// ReconcileAll re-attaches every OIDC-managed client (persisted in the clients
// table) to all inbounds matching XUI_OAUTH_USER_INBOUND_REMARK. Because the roster
// comes from the DB rather than the current inbound membership, an inbound deleted
// and re-added — even the only one a user was on — gets the users back. It only
// attaches (never deletes/disables); subId, credentials, limits and traffic are
// preserved. Returns how many attach operations happened and whether xray needs a restart.
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
	var targets []*model.Inbound
	for _, ib := range inbounds {
		if _, ok := wanted[ib.Remark]; ok {
			targets = append(targets, ib)
		}
	}
	if len(targets) == 0 {
		return 0, false, nil
	}
	targetIDs := make([]int, len(targets))
	present := make(map[string]map[int]struct{})
	for i, ib := range targets {
		targetIDs[i] = ib.Id
		clients, cErr := inboundSvc.GetClients(ib)
		if cErr != nil {
			continue
		}
		for j := range clients {
			email := clients[j].Email
			if email == "" {
				continue
			}
			if present[email] == nil {
				present[email] = make(map[int]struct{})
			}
			present[email][ib.Id] = struct{}{}
		}
	}

	var records []model.ClientRecord
	if err := database.GetDB().Where("oauth_managed = ?", true).Find(&records).Error; err != nil {
		return 0, false, err
	}

	attached := 0
	needRestart := false
	for i := range records {
		rec := records[i]
		on := present[rec.Email]
		var missing []int
		for _, id := range targetIDs {
			if on == nil {
				missing = append(missing, id)
				continue
			}
			if _, ok := on[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			continue
		}
		nr, cErr := clientSvc.Create(inboundSvc, &ClientCreatePayload{Client: clientFromRecord(&rec), InboundIds: missing})
		if cErr != nil {
			logger.Warningf("oauth sync: attach %s to %d inbound(s) failed: %v", rec.Email, len(missing), cErr)
			continue
		}
		attached++
		if nr {
			needRestart = true
		}
	}
	return attached, needRestart, nil
}

// clientFromRecord rebuilds a Client from its persisted record, carrying the
// credentials, subId and limits so a re-attach reuses the identity unchanged.
func clientFromRecord(r *model.ClientRecord) model.Client {
	c := model.Client{
		ID:           r.UUID,
		Email:        r.Email,
		SubID:        r.SubID,
		Password:     r.Password,
		Auth:         r.Auth,
		Flow:         r.Flow,
		Security:     r.Security,
		Secret:       r.Secret,
		PrivateKey:   r.PrivateKey,
		PublicKey:    r.PublicKey,
		PreSharedKey: r.PreSharedKey,
		KeepAlive:    r.KeepAlive,
		Enable:       r.Enable,
		LimitIP:      r.LimitIP,
		TotalGB:      r.TotalGB,
		ExpiryTime:   r.ExpiryTime,
	}
	if r.AllowedIPs != "" {
		c.AllowedIPs = strings.Split(r.AllowedIPs, ",")
	}
	return c
}

// markOauthManaged flags a client's record so ReconcileAll treats it as part of
// the OIDC pool regardless of its current inbound membership.
func markOauthManaged(email string) error {
	return database.GetDB().Model(&model.ClientRecord{}).
		Where("email = ?", email).
		Update("oauth_managed", true).Error
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
		Flow:    cfg.UserFlow,
		LimitIP: cfg.UserLimitIP,
		TotalGB: cfg.UserTotalGB * 1024 * 1024 * 1024,
		SubID:   uuid.NewString(),
	}
	if cfg.UserExpiryDays > 0 {
		c.ExpiryTime = time.Now().Add(time.Duration(cfg.UserExpiryDays) * 24 * time.Hour).UnixMilli()
	}
	return c
}
