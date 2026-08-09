package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// OAuthSyncJob periodically attaches OIDC user-tier clients to inbounds added
// after they logged in, so a new matching inbound reaches existing users without
// a re-login. It only attaches, never removes.
type OAuthSyncJob struct {
	settingService service.SettingService
	inboundService service.InboundService
	clientService  service.ClientService
	xrayService    service.XrayService
	provision      service.OAuthProvisionService
}

// NewOAuthSyncJob creates a new OAuth reconcile job instance.
func NewOAuthSyncJob() *OAuthSyncJob {
	return new(OAuthSyncJob)
}

// Run reconciles user-tier clients across the configured inbound remarks.
func (j *OAuthSyncJob) Run() {
	if !j.settingService.OAuthEnabledEffective() {
		return
	}
	cfg := j.settingService.GetEffectiveOAuthConfig()
	if len(cfg.UserInboundRemarks) == 0 {
		return
	}
	attached, needRestart, err := j.provision.ReconcileAll(&j.inboundService, &j.clientService, cfg)
	if err != nil {
		logger.Warning("oauth sync failed:", err)
		return
	}
	if attached > 0 {
		logger.Infof("oauth sync: attached %d client(s) to newly matched inbounds", attached)
	}
	if needRestart {
		j.xrayService.SetToNeedRestart()
	}
}
