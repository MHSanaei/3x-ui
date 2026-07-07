package job

import (
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	ldaputil "github.com/mhsanaei/3x-ui/v3/internal/util/ldap"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var DefaultTruthyValues = []string{"true", "1", "yes", "on"}

type LdapSyncJob struct {
	settingService service.SettingService
	inboundService service.InboundService
	clientService  service.ClientService
	xrayService    service.XrayService
}

// --- Helper functions for mustGet ---
func mustGetString(fn func() (string, error)) string {
	v, err := fn()
	if err != nil {
		panic(err)
	}
	return v
}

func mustGetInt(fn func() (int, error)) int {
	v, err := fn()
	if err != nil {
		panic(err)
	}
	return v
}

func mustGetBool(fn func() (bool, error)) bool {
	v, err := fn()
	if err != nil {
		panic(err)
	}
	return v
}

func mustGetStringOr(fn func() (string, error), fallback string) string {
	v, err := fn()
	if err != nil || v == "" {
		return fallback
	}
	return v
}

func NewLdapSyncJob() *LdapSyncJob {
	return new(LdapSyncJob)
}

func (j *LdapSyncJob) Run() {
	logger.Info("LDAP sync job started")

	enabled, err := j.settingService.GetLdapEnable()
	if err != nil || !enabled {
		logger.Warning("LDAP disabled or failed to fetch flag")
		return
	}

	// --- LDAP fetch ---
	cfg := ldaputil.Config{
		Host:               mustGetString(j.settingService.GetLdapHost),
		Port:               mustGetInt(j.settingService.GetLdapPort),
		UseTLS:             mustGetBool(j.settingService.GetLdapUseTLS),
		InsecureSkipVerify: mustGetBool(j.settingService.GetLdapInsecureSkipVerify),
		BindDN:             mustGetString(j.settingService.GetLdapBindDN),
		Password:           mustGetString(j.settingService.GetLdapPassword),
		BaseDN:             mustGetString(j.settingService.GetLdapBaseDN),
		UserFilter:         mustGetString(j.settingService.GetLdapUserFilter),
		UserAttr:           mustGetString(j.settingService.GetLdapUserAttr),
		FlagField:          mustGetStringOr(j.settingService.GetLdapFlagField, mustGetString(j.settingService.GetLdapVlessField)),
		TruthyVals:         splitCsv(mustGetString(j.settingService.GetLdapTruthyValues)),
		Invert:             mustGetBool(j.settingService.GetLdapInvertFlag),
	}

	flags, err := ldaputil.FetchVlessFlags(cfg)
	if err != nil {
		logger.Warning("LDAP fetch failed:", err)
		return
	}
	logger.Infof("Fetched %d LDAP flags", len(flags))

	// --- Load all inbounds and all clients once ---
	inboundTags := splitCsv(mustGetString(j.settingService.GetLdapInboundTags))
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Failed to get inbounds:", err)
		return
	}

	allClients := map[string]*model.Client{}  // email -> client
	inboundMap := map[string]*model.Inbound{} // tag -> inbound
	for _, ib := range inbounds {
		inboundMap[ib.Tag] = ib
		clients, _ := j.inboundService.GetClients(ib)
		for i := range clients {
			allClients[clients[i].Email] = &clients[i]
		}
	}

	// --- Prepare batch operations ---
	autoCreate := mustGetBool(j.settingService.GetLdapAutoCreate)
	defGB := mustGetInt(j.settingService.GetLdapDefaultTotalGB)
	defExpiryDays := mustGetInt(j.settingService.GetLdapDefaultExpiryDays)
	defLimitIP := mustGetInt(j.settingService.GetLdapDefaultLimitIP)

	resolvedInboundIds := make([]int, 0, len(inboundTags))
	resolvedTags := make([]string, 0, len(inboundTags))
	for _, tag := range inboundTags {
		ib := inboundMap[tag]
		if ib == nil {
			logger.Warningf("LDAP inbound tag %s does not match any inbound", tag)
			continue
		}
		resolvedInboundIds = append(resolvedInboundIds, ib.Id)
		resolvedTags = append(resolvedTags, tag)
	}

	clientsToCreate := []model.Client{}
	clientsToEnable := map[string][]string{}  // tag -> []email
	clientsToDisable := map[string][]string{} // tag -> []email

	for email, allowed := range flags {
		existing := allClients[email]
		if existing == nil {
			if allowed && autoCreate {
				clientsToCreate = append(clientsToCreate, j.buildClient(email, defGB, defExpiryDays, defLimitIP))
			}
			continue
		}
		for _, tag := range resolvedTags {
			if allowed && !existing.Enable {
				clientsToEnable[tag] = append(clientsToEnable[tag], email)
			} else if !allowed && existing.Enable {
				clientsToDisable[tag] = append(clientsToDisable[tag], email)
			}
		}
	}

	j.createClients(clientsToCreate, resolvedInboundIds, resolvedTags)

	// --- Execute enable/disable batch ---
	for tag, emails := range clientsToEnable {
		j.batchSetEnable(inboundMap[tag], emails, true)
	}
	for tag, emails := range clientsToDisable {
		j.batchSetEnable(inboundMap[tag], emails, false)
	}

	// --- Auto delete clients not in LDAP ---
	autoDelete := mustGetBool(j.settingService.GetLdapAutoDelete)
	if autoDelete {
		ldapEmailSet := map[string]struct{}{}
		for e := range flags {
			ldapEmailSet[e] = struct{}{}
		}
		for _, tag := range inboundTags {
			j.deleteClientsNotInLDAP(tag, ldapEmailSet)
		}
	}
}

func splitCsv(s string) []string {
	if s == "" {
		return DefaultTruthyValues
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// buildClient creates a new client for auto-create; ClientService.Create fills per-protocol credentials
func (j *LdapSyncJob) buildClient(email string, defGB, defExpiryDays, defLimitIP int) model.Client {
	c := model.Client{
		Email:   email,
		Enable:  true,
		LimitIP: defLimitIP,
		TotalGB: int64(defGB),
	}
	if defExpiryDays > 0 {
		c.ExpiryTime = time.Now().Add(time.Duration(defExpiryDays) * 24 * time.Hour).UnixMilli()
	}
	return c
}

// createClients adds each new LDAP client once, attached to every configured inbound
func (j *LdapSyncJob) createClients(newClients []model.Client, inboundIds []int, tags []string) {
	if len(newClients) == 0 || len(inboundIds) == 0 {
		return
	}
	tagList := strings.Join(tags, ",")
	created := 0
	restartNeeded := false
	for _, c := range newClients {
		nr, err := j.clientService.Create(&j.inboundService, &service.ClientCreatePayload{Client: c, InboundIds: inboundIds})
		if err != nil {
			logger.Warningf("Failed to add client %s for tags %s: %v", c.Email, tagList, err)
			continue
		}
		created++
		if nr {
			restartNeeded = true
		}
	}
	if created == 0 {
		return
	}
	logger.Infof("LDAP auto-create: %d clients for %s", created, tagList)
	if restartNeeded {
		j.xrayService.SetToNeedRestart()
	}
}

func (j *LdapSyncJob) batchSetEnable(ib *model.Inbound, emails []string, enable bool) {
	if len(emails) == 0 {
		return
	}
	restartNeeded := false
	changed := 0
	for _, email := range emails {
		ok, needRestart, err := j.clientService.SetClientEnableByEmail(&j.inboundService, email, enable)
		if err != nil {
			logger.Warningf("Batch set enable failed for %s in inbound %s: %v", email, ib.Tag, err)
			continue
		}
		if ok {
			changed++
		}
		if needRestart {
			restartNeeded = true
		}
	}
	if changed > 0 {
		logger.Infof("Batch set enable=%v for %d clients in inbound %s", enable, changed, ib.Tag)
	}
	if restartNeeded {
		j.xrayService.SetToNeedRestart()
	}
}

// deleteClientsNotInLDAP deletes clients not in LDAP using batches and a single restart
func (j *LdapSyncJob) deleteClientsNotInLDAP(inboundTag string, ldapEmails map[string]struct{}) {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Failed to get inbounds for deletion:", err)
		return
	}

	batchSize := 50 //  clients in 1 batch
	restartNeeded := false

	for _, ib := range inbounds {
		if ib.Tag != inboundTag {
			continue
		}
		clients, err := j.inboundService.GetClients(ib)
		if err != nil {
			logger.Warningf("Failed to get clients for inbound %s: %v", ib.Tag, err)
			continue
		}

		// Collect clients for deletion
		toDelete := []model.Client{}
		for _, c := range clients {
			if _, ok := ldapEmails[c.Email]; !ok {
				toDelete = append(toDelete, c)
			}
		}

		if len(toDelete) == 0 {
			continue
		}

		for i := 0; i < len(toDelete); i += batchSize {
			end := min(i+batchSize, len(toDelete))
			batch := toDelete[i:end]

			for _, c := range batch {
				nr, err := j.clientService.DetachByEmail(&j.inboundService, ib.Id, c.Email)
				if err != nil {
					logger.Warningf("Failed to delete client %s from inbound id=%d(tag=%s): %v",
						c.Email, ib.Id, ib.Tag, err)
					continue
				}
				logger.Infof("Deleted client %s from inbound id=%d(tag=%s)",
					c.Email, ib.Id, ib.Tag)
				if nr {
					restartNeeded = true
				}
			}
		}
	}

	if restartNeeded {
		j.xrayService.SetToNeedRestart()
		logger.Info("Xray restart scheduled after batch deletion")
	}
}
