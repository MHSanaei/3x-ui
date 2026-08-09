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
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// OAuthProvisionService turns a verified OIDC user-tier identity into a usable
// VPN connection: it guarantees a Client exists on every inbound whose remark
// matches XUI_OAUTH_USER_INBOUND_REMARK.
type OAuthProvisionService struct{}

// EnsureUserClient returns the subId of the caller's client, creating one on
// every inbound whose remark is listed in XUI_OAUTH_USER_INBOUND_REMARK, with
// the env default limits, on first login. An existing client is reused, not cloned.
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

	if subID, ok := existingSubID(inboundSvc, targets, email); ok {
		return subID, false, nil
	}

	client := buildOAuthClient(email, cfg)
	inboundIds := make([]int, len(targets))
	for i, ib := range targets {
		inboundIds[i] = ib.Id
	}
	needRestart, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{
		Client:     client,
		InboundIds: inboundIds,
	})
	if err != nil {
		// A client with this email may exist on another inbound; reuse its subId
		// rather than fail the login.
		if subID, ok := storedSubIDByEmail(email); ok {
			return subID, false, nil
		}
		return "", needRestart, err
	}
	return client.SubID, needRestart, nil
}

// existingSubID returns the subId of an already-provisioned client found on any
// target inbound, ensuring it is non-empty.
func existingSubID(inboundSvc *InboundService, targets []*model.Inbound, email string) (string, bool) {
	for _, target := range targets {
		clients, err := inboundSvc.GetClients(target)
		if err != nil {
			continue
		}
		for i := range clients {
			if clients[i].Email == email && clients[i].SubID != "" {
				return clients[i].SubID, true
			}
		}
	}
	return "", false
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
